package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/schema"
)

type PricingLoader func(context.Context) (map[string]map[string]any, error)

// Public fields are selected independently; price failures must not poison the executable catalog.
const PricingQuery = `SELECT a.id::text,jsonb_build_object(
 'mode',a.billing_mode,'currency',a.billing_currency,'price_micro_usd',a.price_micro_usd::text,
 'unit',a.billing_unit,'unit_quantity',a.unit_quantity::text,'quantity_scale',a.quantity_scale,
 'rounding',a.rounding,'usage_pointer',a.usage_pointer,
 'charge_condition',jsonb_build_object('source',a.charge_source,'pointer',a.charge_pointer,'op',a.charge_operator,
 'value',CASE WHEN a.charge_operator='eq' THEN COALESCE(to_jsonb(a.charge_text_values),to_jsonb(a.charge_number_values),to_jsonb(a.charge_boolean_values),'[null]'::jsonb)->0
 ELSE COALESCE(to_jsonb(a.charge_text_values),to_jsonb(a.charge_number_values),to_jsonb(a.charge_boolean_values),'[null]'::jsonb) END),
 'policy',CASE WHEN p.api_id IS NULL THEN NULL ELSE jsonb_build_object('policy_version',p.policy_version,'parameters',p.parameters,'sale',p.sale_policy) END)
 FROM public.agent_apis a LEFT JOIN public.agent_api_pricing_policies p ON p.api_id=a.id WHERE a.status='active'`

func PostgresPricingLoader(pool *pgxpool.Pool, timeout time.Duration) PricingLoader {
	return func(ctx context.Context) (map[string]map[string]any, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		rows, err := pool.Query(ctx, PricingQuery)
		if err != nil {
			return nil, fmt.Errorf("public pricing read failed")
		}
		defer rows.Close()
		result := map[string]map[string]any{}
		for rows.Next() {
			var id string
			var raw []byte
			if rows.Scan(&id, &raw) != nil {
				return nil, fmt.Errorf("public pricing decode failed")
			}
			v, err := schema.Decode(raw)
			if err != nil {
				return nil, fmt.Errorf("public pricing decode failed")
			}
			p, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("public pricing decode failed")
			}
			result[id] = p
		}
		if rows.Err() != nil {
			return nil, fmt.Errorf("public pricing read failed")
		}
		return result, nil
	}
}
func object(v any, keys ...string) map[string]any {
	src, _ := v.(map[string]any)
	dst := map[string]any{}
	for _, k := range keys {
		if value, ok := src[k]; ok {
			dst[k] = value
		}
	}
	return dst
}
func objects(v any, project func(any) map[string]any) []any {
	src, _ := v.([]any)
	out := []any{}
	for _, item := range src {
		out = append(out, project(item))
	}
	return out
}
func conditions(v any) []any {
	return objects(v, func(x any) map[string]any { return object(x, "parameter", "op", "value") })
}
func tariff(v any) map[string]any {
	src, _ := v.(map[string]any)
	dst := object(v, "base_price_micro_usd", "rounding", "max_cost_micro_usd")
	for _, key := range []string{"variants", "addons"} {
		dst[key] = objects(src[key], func(x any) map[string]any {
			m, _ := x.(map[string]any)
			o := object(x, "id", "label", "price_micro_usd")
			o["when"] = conditions(m["when"])
			return o
		})
	}
	dst["quantities"] = objects(src["quantities"], func(x any) map[string]any {
		m, _ := x.(map[string]any)
		o := object(x, "id", "label", "parameter", "source", "pointer", "measure", "price_micro_usd", "unit", "unit_quantity", "quantity_scale", "max_quantity", "quantity_rounding")
		o["when"] = conditions(m["when"])
		if m["reservation"] != nil {
			o["reservation"] = object(m["reservation"], "parameter", "measure")
		}
		return o
	})
	return dst
}
func publicPricing(raw map[string]any) map[string]any {
	unavailable := map[string]any{"status": "unavailable"}
	if raw["currency"] != "USD" {
		return unavailable
	}
	mode, _ := raw["mode"].(string)
	if mode != "fixed" && mode != "usage" && mode != "rules" {
		return unavailable
	}
	p := object(raw, "mode", "currency")
	if mode == "rules" {
		policy, _ := raw["policy"].(map[string]any)
		sale, _ := policy["sale"].(map[string]any)
		if sale == nil || sale["base_price_micro_usd"] == nil {
			return unavailable
		}
		public := object(policy, "policy_version")
		public["sale"] = tariff(sale)
		public["parameters"] = objects(policy["parameters"], func(x any) map[string]any {
			return object(x, "id", "location", "pointer", "name", "type", "required", "default")
		})
		p["policy"] = public
	} else {
		price, ok := raw["price_micro_usd"].(string)
		unit, unitOK := raw["unit"].(string)
		if !ok || price == "" || strings.Trim(price, "0123456789") != "" || !unitOK || unit == "" {
			return unavailable
		}
		p["price_micro_usd"], p["unit"] = price, unit
		if mode == "usage" {
			for k, v := range object(raw, "unit_quantity", "quantity_scale", "rounding", "usage_pointer") {
				p[k] = v
			}
		}
	}
	if raw["charge_condition"] != nil {
		p["charge_condition"] = object(raw["charge_condition"], "source", "pointer", "op", "value")
	}
	p["status"] = "available"
	p["settlement"] = "gateway"
	return p
}
