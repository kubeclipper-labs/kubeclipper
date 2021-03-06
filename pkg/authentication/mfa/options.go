/*
 *
 *  * Copyright 2021 KubeClipper Authors.
 *  *
 *  * Licensed under the Apache License, Version 2.0 (the "License");
 *  * you may not use this file except in compliance with the License.
 *  * You may obtain a copy of the License at
 *  *
 *  *     http://www.apache.org/licenses/LICENSE-2.0
 *  *
 *  * Unless required by applicable law or agreed to in writing, software
 *  * distributed under the License is distributed on an "AS IS" BASIS,
 *  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  * See the License for the specific language governing permissions and
 *  * limitations under the License.
 *
 */

package mfa

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/kubeclipper/kubeclipper/pkg/authentication/oauth"
)

func NewOptions() *Options {
	return &Options{
		Enabled:      false,
		MFAProviders: nil,
	}
}

type Options struct {
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	MFAProviders []ProviderOptions `json:"mfaProviders" yaml:"mfaProviders"`
}

type ProviderOptions struct {
	Type    string               `json:"type" yaml:"type"`
	Options oauth.DynamicOptions `json:"options" yaml:"options"`
}

func (a *Options) Validate() []error {
	var errs []error
	if a.Enabled && len(a.MFAProviders) == 0 {
		errs = append(errs, fmt.Errorf("mfa is not configured"))
	}
	return errs
}

func (a *Options) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&a.Enabled, "mfa-enabled", a.Enabled, "Enable multi-factor authentication.")
}
