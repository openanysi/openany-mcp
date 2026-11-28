package seo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Tariff reads the public catalog price for one upstream call.
// is_quote is always false: the gateway computes the charge.
func Tariff(pricing map[string]any, units int) map[string]any {
	out := map[string]any{"is_quote": false, "status": "unavailable", "requested_units": units}
	if pricing == nil {
		out["description"] = "Public pricing is unavailable. Do not infer that this call is free."
		return out
	}
	if status, _ := pricing["status"].(string); status != "" {
		out["status"] = status
	}
	if pricing["status"] != "available" {
		out["description"] = "Public pricing is unavailable. Do not infer that this call is free."
		return out
	}
	out["mode"] = pricing["mode"]
	out["currency"] = pricing["currency"]
	if pricing["charge_condition"] != nil {
		out["charge_condition"] = pricing["charge_condition"]
	}
	if mode, _ := pricing["mode"].(string); mode != "rules" {
		if amount, ok := usd(stringValue(pricing["price_micro_usd"])); ok {
			out["amount_usd"] = amount
			out["unit"] = pricing["unit"]
			out["description"] = "USD " + amount + " from the published catalog price. A displayed price is not a price lock."
		}
		return out
	}
	policy, _ := pricing["policy"].(map[string]any)
	sale, _ := policy["sale"].(map[string]any)
	baseMicro := stringValue(sale["base_price_micro_usd"])
	base, baseOK := micro(baseMicro)
	if amount, ok := usd(baseMicro); ok {
		out["base_usd"] = amount
	}
	if amount, ok := usd(stringValue(sale["max_cost_micro_usd"])); ok {
		out["catalog_max_usd"] = amount
	}
	perResult := 0
	perOK := false
	extra := false
	for _, raw := range anySlice(sale["quantities"]) {
		item, _ := raw.(map[string]any)
		price, ok := micro(stringValue(item["price_micro_usd"]))
		if !ok || price == 0 {
			continue
		}
		pointer := stringValue(item["pointer"])
		if strings.Contains(pointer, "items_count") {
			perResult = price
			perOK = true
			if amount, ok := usd(stringValue(item["price_micro_usd"])); ok {
				out["per_result_usd"] = amount
			}
			continue
		}
		extra = true
	}
	if !baseOK {
		out["description"] = "Rule-based pricing is published, but the base amount could not be read. Do not infer that this call is free. is_quote is false."
		return out
	}
	if extra {
		out["description"] = "Published rule-based pricing has quantities this reading does not add up. Inspect pricing. A displayed price is not a price lock."
		return out
	}
	total := base
	if perOK {
		if units < 0 {
			units = 0
		}
		total += perResult * units
	}
	if maxMicro, ok := micro(stringValue(sale["max_cost_micro_usd"])); ok && total > maxMicro {
		total = maxMicro
	}
	if amount, ok := usd(strconvInt(total)); ok {
		out["published_tariff_maximum_usd"] = amount
		out["published_tariff_maximum_micro_usd"] = strconvInt(total)
	}
	out["description"] = "Published DataForSEO tariff for this request shape. The gateway computes the final charge. A displayed price is not a price lock."
	return out
}

// PriceVersion binds the confirmed request to the catalog versions that
// include the displayed price. Changing the request or the price changes it.
func PriceVersion(calls []Call, versions []string) (string, error) {
	items := make([]any, 0, len(calls))
	for i, call := range calls {
		version := ""
		if i < len(versions) {
			version = versions[i]
		}
		items = append(items, map[string]any{"api": call.API, "api_version": version, "body": call.Tasks})
	}
	raw, err := json.Marshal(map[string]any{"calls": items})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func anySlice(v any) []any {
	items, _ := v.([]any)
	return items
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func micro(s string) (int, bool) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n, true
}

func usd(microUSD string) (string, bool) {
	if _, ok := micro(microUSD); !ok {
		return "", false
	}
	for len(microUSD) <= 6 {
		microUSD = "0" + microUSD
	}
	whole, frac := microUSD[:len(microUSD)-6], strings.TrimRight(microUSD[len(microUSD)-6:], "0")
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if frac == "" {
		return whole, true
	}
	return whole + "." + frac, true
}

func strconvInt(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
