package evaluation

import (
	"context"
	"errors"
)

var ErrPinnedArtifactUnavailable = errors.New("pinned replay artifact unavailable")

// StaticPinValidator is useful for hermetic replay where artifacts are already
// materialized by the caller. It still validates every identity exactly.
type StaticPinValidator struct{ Available Manifest }

func (validator StaticPinValidator) ValidatePins(_ context.Context, manifest Manifest) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	available := validator.Available
	if manifest.CatalogSnapshot != available.CatalogSnapshot || manifest.PolicyRevision != available.PolicyRevision || manifest.ProtocolSchema != available.ProtocolSchema || manifest.NormalizationVersion != available.NormalizationVersion || manifest.IndexVersion != available.IndexVersion || manifest.FactProviderFixture != available.FactProviderFixture || manifest.FactProviderVersion != available.FactProviderVersion || manifest.Binary != available.Binary {
		return ErrPinnedArtifactUnavailable
	}
	if (manifest.Model == nil) != (available.Model == nil) || manifest.Model != nil && *manifest.Model != *available.Model {
		return ErrPinnedArtifactUnavailable
	}
	return nil
}
