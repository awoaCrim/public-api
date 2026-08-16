package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// SaveReviewSetting persists review settings to the option table via the bulk
// updater. A plaintext api_key in the updates is encrypted first; an empty or
// masked key leaves the stored key untouched.
func SaveReviewSetting(updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}
	if plain, ok := updates["llm_review_setting.api_key"]; ok && plain != "" {
		enc, err := EncryptLLMReviewAPIKey(plain)
		if err != nil {
			return err
		}
		updates["llm_review_setting.api_key"] = enc
	}
	if model.DB == nil {
		return errors.New("main database is not initialized")
	}
	return model.UpdateOptionsBulk(updates)
}

// SaveReviewSchemaTestFailure records and persists an explicit capability
// test failure (masked).
func SaveReviewSchemaTestFailure(errMsg string) error {
	operation_setting.ResetSchemaCapability()
	operation_setting.MarkReviewTestError(errMsg)
	return SaveReviewSetting(map[string]string{
		"llm_review_setting.schema_tested":                  "false",
		"llm_review_setting.schema_tested_at":               "0",
		"llm_review_setting.schema_tested_model":            "",
		"llm_review_setting.schema_version":                 "",
		"llm_review_setting.structured_output_tested":       "false",
		"llm_review_setting.structured_output_tested_at":    "0",
		"llm_review_setting.structured_output_tested_model": "",
		"llm_review_setting.structured_output_version":      "",
		"llm_review_setting.test_error":                     common.MaskReviewCredentialText(common.MaskSensitiveInfo(errMsg)),
	})
}
