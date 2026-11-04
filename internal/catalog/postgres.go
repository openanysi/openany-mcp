package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/model"
	"time"
)

const Query = `SELECT a.id::text,a.name,COALESCE(a.description,''),a.provider_id,a.public_mcp_tool_name,a.method,a.path_template,a.public_path,a.status,a.revision,a.upstream_protocol,a.request_content_type,a.unknown_parameters,a.billing_mode,c.api_id IS NOT NULL,c.inputs FROM public.agent_apis a LEFT JOIN public.agent_api_request_configs c ON c.api_id=a.id WHERE a.status='active' ORDER BY a.id`

func PostgresLoader(pool *pgxpool.Pool, timeout time.Duration) Loader {
	return func(ctx context.Context) ([]model.Record, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		rows, err := pool.Query(ctx, Query)
		if err != nil {
			return nil, fmt.Errorf("catalog database query failed")
		}
		defer rows.Close()
		result := []model.Record{}
		for rows.Next() {
			var r model.Record
			var inputs []byte
			if err := rows.Scan(&r.APIID, &r.Name, &r.Description, &r.ProviderID, &r.ToolName, &r.Method, &r.PathTemplate, &r.PublicPath, &r.Status, &r.Revision, &r.UpstreamProtocol, &r.RequestContentType, &r.UnknownParameters, &r.BillingMode, &r.ConfigPresent, &inputs); err != nil {
				return nil, fmt.Errorf("catalog database row decode failed")
			}
			if r.ConfigPresent {
				if len(inputs) == 0 || string(inputs) == "null" {
					r.InputError = "null inputs"
				} else if err = json.Unmarshal(inputs, &r.Inputs); err != nil {
					r.InputError = "invalid inputs"
					r.Inputs = nil
				}
			}
			result = append(result, r)
		}
		if rows.Err() != nil {
			return nil, fmt.Errorf("catalog database read failed")
		}
		return result, nil
	}
}
