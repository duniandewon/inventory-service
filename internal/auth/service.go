package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/duniandewon/inventory-service/internal/config"
)

type Service struct {
	repo      *Repository
	otps      OTPStore
	tokens    TokenStore
	sender    OTPSender
	jwtSecret string
	env       *config.Env
}

func NewService(repo *Repository, otps OTPStore, tokens TokenStore, sender OTPSender, env *config.Env) *Service {
	return &Service{
		repo:      repo,
		otps:      otps,
		tokens:    tokens,
		sender:    sender,
		jwtSecret: env.JwtSecret,
		env:       env,
	}
}

func generateOTPCode(length int) (string, error) {
	digits := make([]byte, length)
	for i := range digits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generating otp digit: %w", err)
		}
		digits[i] = byte('0' + n.Int64())
	}
	return string(digits), nil
}

func hashOTPCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) RequestOTP(ctx context.Context, phone string) error {
	lockedOut, err := s.otps.IsLockedOut(ctx, phone)
	if err != nil {
		return err
	}
	if lockedOut {
		return ErrLockedOut
	}

	count, err := s.otps.IncrRequestCount(ctx, phone, s.env.OTPRequestWindow)
	if err != nil {
		return err
	}
	if count > s.env.OTPMaxRequests {
		if err := s.otps.SetLockout(ctx, phone, s.env.OTPLockoutTTL); err != nil {
			return err
		}
		return ErrTooManyRequests
	}

	user, err := s.repo.FindActiveUserByPhone(ctx, phone)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	code, err := generateOTPCode(s.env.OTPLength)
	if err != nil {
		return err
	}

	if err := s.otps.SaveCode(ctx, phone, hashOTPCode(code), s.env.OTPTTL); err != nil {
		return err
	}

	if err := s.sender.SendOTP(ctx, user.PhoneNumber, code); err != nil {
		return fmt.Errorf("sending otp: %w", err)
	}

	return nil
}

func (s *Service) VerifyOTP(ctx context.Context, phone, code string) (*TokenPair, error) {
	lockedOut, err := s.otps.IsLockedOut(ctx, phone)
	if err != nil {
		return nil, err
	}
	if lockedOut {
		return nil, ErrLockedOut
	}

	codeHash, attempts, err := s.otps.GetCode(ctx, phone)
	if err != nil {
		return nil, err
	}
	if attempts >= s.env.OTPMaxAttempts {
		if err := s.lockOutAndClear(ctx, phone); err != nil {
			return nil, err
		}
		return nil, ErrLockedOut
	}

	if subtle.ConstantTimeCompare([]byte(codeHash), []byte(hashOTPCode(code))) != 1 {
		attempts, err := s.otps.IncrAttempts(ctx, phone)
		if err != nil {
			return nil, err
		}
		if attempts >= s.env.OTPMaxAttempts {
			if err := s.lockOutAndClear(ctx, phone); err != nil {
				return nil, err
			}
		}
		return nil, ErrOTPMismatch
	}

	if err := s.otps.DeleteCode(ctx, phone); err != nil {
		return nil, err
	}

	user, err := s.repo.FindActiveUserByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}

	roles, err := s.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, user.ID, roles)
}

func (s *Service) lockOutAndClear(ctx context.Context, phone string) error {
	if err := s.otps.DeleteCode(ctx, phone); err != nil {
		return err
	}
	return s.otps.SetLockout(ctx, phone, s.env.OTPLockoutTTL)
}

func (s *Service) issueTokenPair(ctx context.Context, userID int, roles []string) (*TokenPair, error) {
	access, _, err := signToken(s.jwtSecret, userID, roles, tokenTypeAccess, s.env.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	refresh, jti, err := signToken(s.jwtSecret, userID, nil, tokenTypeRefresh, s.env.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	if err := s.tokens.SaveRefreshToken(ctx, jti, userID, s.env.RefreshTokenTTL); err != nil {
		return nil, err
	}
	if err := s.tokens.AddUserSession(ctx, userID, jti, s.env.RefreshTokenTTL); err != nil {
		return nil, err
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := parseToken(s.jwtSecret, refreshToken, tokenTypeRefresh)
	if err != nil {
		return nil, ErrRefreshNotFound
	}
	jti := claims.ID

	userID, err := s.tokens.GetRefreshUserID(ctx, jti)
	if errors.Is(err, ErrRefreshNotFound) {
		if rotatedUserID, wasRotated, rotatedErr := s.tokens.GetRotatedUserID(ctx, jti); rotatedErr == nil && wasRotated {
			if revokeErr := s.tokens.RevokeAllUserSessions(ctx, rotatedUserID); revokeErr != nil {
				return nil, revokeErr
			}
			return nil, ErrRefreshTokenReuse
		}
		return nil, ErrRefreshNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := s.tokens.DeleteRefreshToken(ctx, jti); err != nil {
		return nil, err
	}
	if err := s.tokens.MarkRotated(ctx, jti, userID, s.env.RefreshTokenTTL); err != nil {
		return nil, err
	}
	if err := s.tokens.RemoveUserSession(ctx, userID, jti); err != nil {
		return nil, err
	}

	roles, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, userID, roles)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := parseToken(s.jwtSecret, refreshToken, tokenTypeRefresh)
	if err != nil {
		return nil
	}
	jti := claims.ID

	userID, err := s.tokens.GetRefreshUserID(ctx, jti)
	if errors.Is(err, ErrRefreshNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := s.tokens.DeleteRefreshToken(ctx, jti); err != nil {
		return err
	}
	return s.tokens.RemoveUserSession(ctx, userID, jti)
}

func (s *Service) Me(ctx context.Context, userID int) (*User, []string, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	roles, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return user, roles, nil
}

func (s *Service) ParseAccessToken(accessToken string) (userID int, roles []string, err error) {
	claims, err := parseToken(s.jwtSecret, accessToken, tokenTypeAccess)
	if err != nil {
		return 0, nil, err
	}
	id, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return 0, nil, fmt.Errorf("parsing access token subject: %w", err)
	}
	return id, claims.Roles, nil
}
