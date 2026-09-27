package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var nonDigit = regexp.MustCompile(`\D`)

// normalizePhoneTR strips formatting and puts a Turkish mobile number into
// the international digits-only form wa.me expects (countrycode+number,
// no leading +, no leading 0). Numbers that already look international
// (start with a country code longer than a local 0-prefixed number) are
// left as-is aside from stripping non-digits.
func normalizePhoneTR(phone string) string {
	digits := nonDigit.ReplaceAllString(phone, "")
	switch {
	case digits == "":
		return ""
	case strings.HasPrefix(digits, "90") && len(digits) == 12:
		return digits
	case strings.HasPrefix(digits, "0") && len(digits) == 11:
		return "90" + digits[1:]
	case len(digits) == 10:
		return "90" + digits
	default:
		return digits
	}
}

// whatsAppLink builds a click-to-chat wa.me URL with a prefilled message.
// Returns "" if the phone number is empty, so callers can hide the button.
func whatsAppLink(phone, message string) string {
	number := normalizePhoneTR(phone)
	if number == "" {
		return ""
	}
	return fmt.Sprintf("https://wa.me/%s?text=%s", number, url.QueryEscape(message))
}

// buildReminderMessage composes the Turkish reminder text for a member's
// overdue periods, addressed to the guardian when one is on file.
func buildReminderMessage(m Member, overdue []DuePeriod, total float64) string {
	var months []string
	for _, p := range overdue {
		months = append(months, periodLabelText(p.Period))
	}
	monthsText := strings.Join(months, ", ")

	subject := m.FullName
	if m.GuardianName != "" {
		return fmt.Sprintf(
			"Sayın %s, %s için %s ayına ait toplam %.2f ₺ tutarındaki aidat ödemesi gecikmiştir. Bilgilerinize sunarız.",
			m.GuardianName, subject, monthsText, total,
		)
	}
	return fmt.Sprintf(
		"Sayın %s, %s ayına ait toplam %.2f ₺ tutarındaki aidat ödemeniz gecikmiştir. Bilgilerinize sunarız.",
		subject, monthsText, total,
	)
}
