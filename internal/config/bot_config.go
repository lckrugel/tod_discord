package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

type BotConfig struct {
	BotName    string
	secret_key string
	intents    uint64
}

func NewBotConfig() (*BotConfig, error) {
	discord_api_key, isSet := os.LookupEnv("DISCORD_API_KEY")
	if !isSet {
		return nil, errors.New("missing environment variable: 'DISCORD_API_KEY'")
	}

	file, err := os.Open("config/bot_config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to open 'bot_config.json': %w", err)
	}
	defer file.Close()

	var botConfigFile botConfigFile

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&botConfigFile); err != nil {
		return nil, fmt.Errorf("failed to decode 'bot_config.json': %w", err)
	}

	return &BotConfig{
		BotName:    botConfigFile.BotName,
		secret_key: discord_api_key,
		intents:    botConfigFile.calculateIntents(),
	}, nil
}

func (cfg BotConfig) GetSecretKey() string {
	return cfg.secret_key
}

func (cfg BotConfig) GetIntents() uint64 {
	return cfg.intents
}

type botConfigFile struct {
	BotName string          `json:"bot_name"`
	Intents map[string]bool `json:"intents"`
}

func (bcfg *botConfigFile) calculateIntents() uint64 {
	// All intents and their values
	intentValues := map[string]uint64{
		"guilds":                        1 << 0,
		"guid_members":                  1 << 1,
		"guild_moderation":              1 << 2,
		"guild_expressions":             1 << 3,
		"guild_integrations":            1 << 4,
		"guild_webhooks":                1 << 5,
		"guild_invites":                 1 << 6,
		"guild_voice_states":            1 << 7,
		"guild_presences":               1 << 8,
		"guild_messages":                1 << 9,
		"guild_message_reactions":       1 << 10,
		"guild_message_typing":          1 << 11,
		"direct_messages":               1 << 12,
		"direct_message_reactions":      1 << 13,
		"direct_message_typing":         1 << 14,
		"message_content":               1 << 15,
		"guild_scheduled_events":        1 << 16,
		"auto_moderation_configuration": 1 << 20,
		"auto_moderation_execution":     1 << 21,
		"guild_message_polls":           1 << 24,
		"direct_message_polls":          1 << 25,
	}

	var intents uint64 = 0
	for key, value := range bcfg.Intents {
		if value {
			if bitValue, exists := intentValues[key]; exists {
				intents |= bitValue
			} else {
				slog.Warn("unkown intent key", "code", "W_BAD_CONFIG", "key", key)
			}
		}
	}

	return intents
}
