package worker

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/google/uuid"

	"github.com/hkizilbulak/haradan-be/internal/domain/apperr"
	domainmedia "github.com/hkizilbulak/haradan-be/internal/domain/media"
)

type validatePayload struct {
	AssetID string `json:"assetId"`
}

type variantPayload struct {
	AssetID          string `json:"assetId"`
	TransformProfile string `json:"transformProfile"`
}

type parsedJob struct {
	JobType domainmedia.JobType
	AssetID uuid.UUID
	Profile string
}

func parseMediaJob(jobType domainmedia.JobType, payload json.RawMessage) (parsedJob, error) {
	switch jobType {
	case domainmedia.JobValidateAndNormalize:
		var raw map[string]json.RawMessage
		if err := decodeSingleJSONValue(payload, &raw); err != nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if len(raw) != 1 {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if _, ok := raw["assetId"]; !ok {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		var p validatePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		id, err := uuid.Parse(p.AssetID)
		if err != nil || id == uuid.Nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		return parsedJob{JobType: jobType, AssetID: id}, nil

	case domainmedia.JobGenerateVariant:
		var raw map[string]json.RawMessage
		if err := decodeSingleJSONValue(payload, &raw); err != nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if len(raw) != 2 {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if _, ok := raw["assetId"]; !ok {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if _, ok := raw["transformProfile"]; !ok {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		var p variantPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		id, err := uuid.Parse(p.AssetID)
		if err != nil || id == uuid.Nil {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		if !domainmedia.IsKnownTransformProfile(p.TransformProfile) {
			return parsedJob{}, apperr.Validation(safePayloadErrorMessage)
		}
		return parsedJob{JobType: jobType, AssetID: id, Profile: p.TransformProfile}, nil

	default:
		return parsedJob{}, apperr.Validation(safeUnsupportedJobMessage)
	}
}

// decodeSingleJSONValue unmarshals exactly one JSON value and rejects trailing tokens.
func decodeSingleJSONValue(payload json.RawMessage, dest any) error {
	if len(bytes.TrimSpace(payload)) == 0 {
		return io.ErrUnexpectedEOF
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(dest); err != nil {
		return err
	}
	if dec.More() {
		return errTrailingJSON
	}
	return nil
}

var errTrailingJSON = io.ErrUnexpectedEOF
