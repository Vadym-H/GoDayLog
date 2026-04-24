package domain

// Identity locates a user inside an external provider (Telegram, future REST, etc.).
// Domain tables never store provider-specific IDs; they live here and in user_identities.
type Identity struct {
	Provider   string
	ExternalID string
}
