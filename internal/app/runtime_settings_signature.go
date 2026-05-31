package app

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/byte-v-forge/common-lib/protojsonx"
)

func runtimeSettingsSignature(settings *runtimeSettingsFile) string {
	data, _ := protojsonx.Marshal(normalizeRuntimeSettings(settings))
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
