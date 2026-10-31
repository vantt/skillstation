package catalog

import (
	"context"
	"database/sql"
)

// Resource is one indexed file of a skill.
type Resource struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"size_bytes"`
}

// skillResourcesQuery reads one skill's resources through the resources_skill index.
const skillResourcesQuery = `SELECT path,kind,digest,size_bytes FROM resources WHERE skill_id=? ORDER BY path`

// SkillResources lists the indexed resources of one skill in path order. A
// skill without resources yields an empty list.
func SkillResources(ctx context.Context, database *sql.DB, skillID string) ([]Resource, error) {
	rows, err := database.QueryContext(ctx, skillResourcesQuery, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := []Resource{}
	for rows.Next() {
		var resource Resource
		if err := rows.Scan(&resource.Path, &resource.Kind, &resource.Digest, &resource.SizeBytes); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return resources, nil
}
