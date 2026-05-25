// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/alertmanager/template"
)

// postableAlertV2 matches the Alertmanager v2 POST /api/v2/alerts JSON shape.
type postableAlertV2 struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	StartsAt     time.Time         `json:"startsAt,omitempty"`
	EndsAt       time.Time         `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}

func templateAlertsToPostableV2(alerts template.Alerts) ([]postableAlertV2, error) {
	out := make([]postableAlertV2, 0, len(alerts))
	for _, alt := range alerts {
		labels, err := sanitizeKV(alt.Labels)
		if err != nil {
			return nil, fmt.Errorf("alert labels: %w", err)
		}

		pa := postableAlertV2{
			Labels:   labels,
			StartsAt: alt.StartsAt,
			EndsAt:   alt.EndsAt,
		}
		if alt.GeneratorURL != "" {
			pa.GeneratorURL = alt.GeneratorURL
		}
		if len(alt.Annotations) > 0 {
			annotations, err := sanitizeKV(alt.Annotations)
			if err != nil {
				return nil, fmt.Errorf("alert annotations: %w", err)
			}
			pa.Annotations = annotations
		}
		out = append(out, pa)
	}
	return out, nil
}

func marshalPostableAlertsV2(alerts template.Alerts) ([]byte, error) {
	pAlerts, err := templateAlertsToPostableV2(alerts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(pAlerts)
}
