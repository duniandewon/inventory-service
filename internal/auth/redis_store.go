package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type OTPStore interface {
	IsLockedOut(ctx context.Context, phone string) (bool, error)
	SetLockout(ctx context.Context, phone string, ttl time.Duration) error
	IncrRequestCount(ctx context.Context, phone string, window time.Duration) (int, error)
	SaveCode(ctx context.Context, phone, codeHash string, ttl time.Duration) error
	GetCode(ctx context.Context, phone string) (codeHash string, attempts int, err error)
	IncrAttempts(ctx context.Context, phone string) (int, error)
	DeleteCode(ctx context.Context, phone string) error
}

type TokenStore interface {
	SaveRefreshToken(ctx context.Context, tokenID string, userID int, ttl time.Duration) error
	GetRefreshUserID(ctx context.Context, tokenID string) (int, error)
	DeleteRefreshToken(ctx context.Context, tokenID string) error
	MarkRotated(ctx context.Context, tokenID string, userID int, ttl time.Duration) error
	GetRotatedUserID(ctx context.Context, tokenID string) (int, bool, error)
	AddUserSession(ctx context.Context, userID int, tokenID string, ttl time.Duration) error
	RemoveUserSession(ctx context.Context, userID int, tokenID string) error
	RevokeAllUserSessions(ctx context.Context, userID int) error
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func otpCodeKey(phone string) string     { return "otp:code:" + phone }
func otpRequestsKey(phone string) string { return "otp:requests:" + phone }
func otpLockoutKey(phone string) string  { return "otp:lockout:" + phone }
func refreshKey(tokenID string) string   { return "refresh:" + tokenID }
func rotatedKey(tokenID string) string   { return "refresh:rotated:" + tokenID }
func userSessionsKey(userID int) string  { return "user_sessions:" + strconv.Itoa(userID) }

func (s *RedisStore) IsLockedOut(ctx context.Context, phone string) (bool, error) {
	n, err := s.client.Exists(ctx, otpLockoutKey(phone)).Result()
	if err != nil {
		return false, fmt.Errorf("checking otp lockout: %w", err)
	}
	return n > 0, nil
}

func (s *RedisStore) SetLockout(ctx context.Context, phone string, ttl time.Duration) error {
	if err := s.client.Set(ctx, otpLockoutKey(phone), "1", ttl).Err(); err != nil {
		return fmt.Errorf("setting otp lockout: %w", err)
	}
	return nil
}

func (s *RedisStore) IncrRequestCount(ctx context.Context, phone string, window time.Duration) (int, error) {
	key := otpRequestsKey(phone)
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("incrementing otp request count: %w", err)
	}
	if count == 1 {
		if err := s.client.Expire(ctx, key, window).Err(); err != nil {
			return 0, fmt.Errorf("setting otp request window: %w", err)
		}
	}
	return int(count), nil
}

func (s *RedisStore) SaveCode(ctx context.Context, phone, codeHash string, ttl time.Duration) error {
	key := otpCodeKey(phone)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, "code_hash", codeHash, "attempts", 0)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("saving otp code: %w", err)
	}
	return nil
}

func (s *RedisStore) GetCode(ctx context.Context, phone string) (string, int, error) {
	res, err := s.client.HGetAll(ctx, otpCodeKey(phone)).Result()
	if err != nil {
		return "", 0, fmt.Errorf("reading otp code: %w", err)
	}
	codeHash, ok := res["code_hash"]
	if !ok {
		return "", 0, ErrOTPNotFound
	}
	attempts, _ := strconv.Atoi(res["attempts"])
	return codeHash, attempts, nil
}

func (s *RedisStore) IncrAttempts(ctx context.Context, phone string) (int, error) {
	n, err := s.client.HIncrBy(ctx, otpCodeKey(phone), "attempts", 1).Result()
	if err != nil {
		return 0, fmt.Errorf("incrementing otp attempts: %w", err)
	}
	return int(n), nil
}

func (s *RedisStore) DeleteCode(ctx context.Context, phone string) error {
	if err := s.client.Del(ctx, otpCodeKey(phone)).Err(); err != nil {
		return fmt.Errorf("deleting otp code: %w", err)
	}
	return nil
}

func (s *RedisStore) SaveRefreshToken(ctx context.Context, tokenID string, userID int, ttl time.Duration) error {
	if err := s.client.Set(ctx, refreshKey(tokenID), userID, ttl).Err(); err != nil {
		return fmt.Errorf("saving refresh token: %w", err)
	}
	return nil
}

func (s *RedisStore) GetRefreshUserID(ctx context.Context, tokenID string) (int, error) {
	val, err := s.client.Get(ctx, refreshKey(tokenID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrRefreshNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("reading refresh token: %w", err)
	}
	userID, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parsing refresh token user id: %w", err)
	}
	return userID, nil
}

func (s *RedisStore) DeleteRefreshToken(ctx context.Context, tokenID string) error {
	if err := s.client.Del(ctx, refreshKey(tokenID)).Err(); err != nil {
		return fmt.Errorf("deleting refresh token: %w", err)
	}
	return nil
}

func (s *RedisStore) MarkRotated(ctx context.Context, tokenID string, userID int, ttl time.Duration) error {
	if err := s.client.Set(ctx, rotatedKey(tokenID), userID, ttl).Err(); err != nil {
		return fmt.Errorf("marking refresh token rotated: %w", err)
	}
	return nil
}

func (s *RedisStore) GetRotatedUserID(ctx context.Context, tokenID string) (int, bool, error) {
	val, err := s.client.Get(ctx, rotatedKey(tokenID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("reading rotated refresh token: %w", err)
	}
	userID, err := strconv.Atoi(val)
	if err != nil {
		return 0, false, fmt.Errorf("parsing rotated refresh token user id: %w", err)
	}
	return userID, true, nil
}

func (s *RedisStore) AddUserSession(ctx context.Context, userID int, tokenID string, ttl time.Duration) error {
	key := userSessionsKey(userID)
	pipe := s.client.TxPipeline()
	pipe.SAdd(ctx, key, tokenID)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("adding user session: %w", err)
	}
	return nil
}

func (s *RedisStore) RemoveUserSession(ctx context.Context, userID int, tokenID string) error {
	if err := s.client.SRem(ctx, userSessionsKey(userID), tokenID).Err(); err != nil {
		return fmt.Errorf("removing user session: %w", err)
	}
	return nil
}

func (s *RedisStore) RevokeAllUserSessions(ctx context.Context, userID int) error {
	key := userSessionsKey(userID)
	tokenIDs, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("listing user sessions: %w", err)
	}

	pipe := s.client.TxPipeline()
	for _, tokenID := range tokenIDs {
		pipe.Del(ctx, refreshKey(tokenID))
	}
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoking user sessions: %w", err)
	}
	return nil
}
