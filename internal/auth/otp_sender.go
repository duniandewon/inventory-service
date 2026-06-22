package auth

import (
	"context"
	"log"
)

// OTPSender delivers a one-time code to a phone number. Auth depends on
// this interface, not a concrete WhatsApp client, so the provider can be
// swapped without touching the service logic.
type OTPSender interface {
	SendOTP(ctx context.Context, phone, code string) error
}

// LoggingOTPSender logs the code instead of sending it. Stand-in until a
// real WhatsApp Business API client is wired up.
type LoggingOTPSender struct{}

func NewLoggingOTPSender() *LoggingOTPSender {
	return &LoggingOTPSender{}
}

func (s *LoggingOTPSender) SendOTP(ctx context.Context, phone, code string) error {
	log.Printf("[otp] would send code %s to %s via WhatsApp", code, phone)
	return nil
}
