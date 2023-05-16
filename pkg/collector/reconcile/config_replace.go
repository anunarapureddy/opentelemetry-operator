// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconcile

import (
	"time"

	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Package install has the side-effect of registering all builtin.
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/open-telemetry/opentelemetry-operator/pkg/collector/adapters"
	"github.com/open-telemetry/opentelemetry-operator/pkg/featuregate"
	"github.com/open-telemetry/opentelemetry-operator/pkg/naming"
	ta "github.com/open-telemetry/opentelemetry-operator/pkg/targetallocator/adapters"
)

type targetAllocator struct {
	Endpoint    string        `yaml:"endpoint"`
	Interval    time.Duration `yaml:"interval"`
	CollectorID string        `yaml:"collector_id"`
}

type Config struct {
	PromConfig        *promconfig.Config `yaml:"config"`
	TargetAllocConfig *targetAllocator   `yaml:"target_allocator,omitempty"`
}

func ReplaceConfig(instance v1alpha1.OpenTelemetryCollector) (string, error) {
	// Check if TargetAllocator is enabled, if not, return the original config
	if !instance.Spec.TargetAllocator.Enabled {
		return instance.Spec.Config, nil
	}

	config, err := adapters.ConfigFromString(instance.Spec.Config)
	if err != nil {
		return "", err
	}

	if featuregate.EnableTargetAllocatorRewrite.IsEnabled() {
		// To avoid issues caused by Prometheus validation logic, which fails regex validation when it encounters
		// $$ in the prom config, we update the YAML file directly without marshaling and unmarshalling.
		promCfgMap, getCfgPromErr := ta.AddTAConfigToPromConfig(instance.Spec.Config, naming.TAService(instance))
		if getCfgPromErr != nil {
			return "", getCfgPromErr
		}

		// type coercion checks are handled in the AddTAConfigToPromConfig method above
		config["receivers"].(map[interface{}]interface{})["prometheus"] = promCfgMap

		out, updCfgMarshalErr := yaml.Marshal(config)
		if updCfgMarshalErr != nil {
			return "", updCfgMarshalErr
		}

		return string(out), nil
	}

	// To avoid issues caused by Prometheus validation logic, which fails regex validation when it encounters
	// $$ in the prom config, we update the YAML file directly without marshaling and unmarshalling.
	promCfgMap, err := ta.AddHTTPSDConfigToPromConfig(instance.Spec.Config, naming.TAService(instance))
	if err != nil {
		return "", err
	}

	// type coercion checks are handled in the ConfigToPromConfig method above
	config["receivers"].(map[interface{}]interface{})["prometheus"] = promCfgMap

	out, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
