// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"fmt"

	"github.com/prometheus/alertmanager/matcher/compat"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
)

// sanitizeKV converts template key/value pairs to a label set that satisfies
// Alertmanager's UTF-8 label validation (same rules as POST /api/v2/alerts).
func sanitizeKV(kvs template.KV) (map[string]string, error) {
	if len(kvs) == 0 {
		return nil, fmt.Errorf("empty label set")
	}

	out := make(map[string]string, len(kvs))
	for k, v := range kvs {
		ln := model.LabelName(k)
		lv := model.LabelValue(v)
		if !compat.IsValidLabelName(ln) {
			return nil, fmt.Errorf("invalid label name %q", k)
		}
		if !lv.IsValid() {
			return nil, fmt.Errorf("invalid label value for %q: %q", k, v)
		}
		out[k] = v
	}
	return out, nil
}
