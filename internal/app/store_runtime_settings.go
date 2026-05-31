package app

import (
	"context"
	"errors"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/jackc/pgx/v5"
)

const runtimeSettingsKey = "runtime"

func (s *PostgresStore) LoadRuntimeSettings(ctx context.Context) (*runtimeSettingsFile, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `SELECT setting_json::text FROM proxy_runtime_settings WHERE setting_key=$1`, runtimeSettingsKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return normalizeRuntimeSettings(nil), nil
	}
	if err != nil {
		return nil, err
	}
	settings := &proxyruntimev1.ProxyRuntimePersistentSettings{}
	if raw != "" {
		_ = protojsonx.Unmarshal([]byte(raw), settings)
	}
	return normalizeRuntimeSettings(settings), nil
}

func (s *PostgresStore) SaveRuntimeSettings(ctx context.Context, settings *runtimeSettingsFile) error {
	data, err := protojsonx.Marshal(normalizeRuntimeSettings(settings))
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO proxy_runtime_settings (setting_key, setting_json) VALUES ($1,$2::jsonb) ON CONFLICT (setting_key) DO UPDATE SET setting_json=EXCLUDED.setting_json, updated_at=now()`, runtimeSettingsKey, string(data))
	return err
}
