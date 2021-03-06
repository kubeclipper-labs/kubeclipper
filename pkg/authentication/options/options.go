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

package options

import (
	"errors"
	"time"

	"github.com/spf13/pflag"

	"github.com/kubeclipper/kubeclipper/pkg/authentication/identityprovider"
	"github.com/kubeclipper/kubeclipper/pkg/authentication/mfa"
	"github.com/kubeclipper/kubeclipper/pkg/authentication/oauth"
)

type AuthenticationOptions struct {
	AuthenticateRateLimiterMaxTries int            `json:"authenticateRateLimiterMaxTries" yaml:"authenticateRateLimiterMaxTries"`
	AuthenticateRateLimiterDuration time.Duration  `json:"authenticateRateLimiterDuration" yaml:"authenticateRateLimiterDuration"`
	MaximumClockSkew                time.Duration  `json:"maximumClockSkew" yaml:"maximumClockSkew"`
	LoginHistoryRetentionPeriod     time.Duration  `json:"loginHistoryRetentionPeriod" yaml:"loginHistoryRetentionPeriod"`
	LoginHistoryMaximumEntries      int            `json:"loginHistoryMaximumEntries" yaml:"loginHistoryMaximumEntries"`
	MultipleLogin                   bool           `json:"multipleLogin" yaml:"multipleLogin"`
	MFAOptions                      *mfa.Options   `json:"mfaOptions" yaml:"mfaOptions"`
	JwtSecret                       string         `json:"-" yaml:"jwtSecret"`
	OAuthOptions                    *oauth.Options `json:"oauthOptions" yaml:"oauthOptions"`
}

func NewAuthenticateOptions() *AuthenticationOptions {
	return &AuthenticationOptions{
		AuthenticateRateLimiterMaxTries: 5,
		AuthenticateRateLimiterDuration: time.Minute * 30,
		MaximumClockSkew:                10 * time.Second,
		LoginHistoryRetentionPeriod:     time.Hour * 24 * 7,
		LoginHistoryMaximumEntries:      100,
		MFAOptions:                      mfa.NewOptions(),
		OAuthOptions:                    oauth.NewOauthOptions(),
		MultipleLogin:                   false,
		JwtSecret:                       "kubeclipper",
	}
}

func (a *AuthenticationOptions) Validate() []error {
	var errs []error
	if len(a.JwtSecret) == 0 {
		errs = append(errs, errors.New("JWT secret MUST not be empty"))
	}
	if a.AuthenticateRateLimiterMaxTries > a.LoginHistoryMaximumEntries {
		errs = append(errs, errors.New("authenticateRateLimiterMaxTries MUST not be greater than loginHistoryMaximumEntries"))
	}
	if err := identityprovider.SetupWithOptions(a.OAuthOptions.IdentityProviders); err != nil {
		errs = append(errs, err)
	}
	return errs
}

func (a *AuthenticationOptions) AddFlags(fs *pflag.FlagSet) {
	a.MFAOptions.AddFlags(fs)
	fs.IntVar(&a.AuthenticateRateLimiterMaxTries, "authenticate-rate-limiter-max-retries", a.AuthenticateRateLimiterMaxTries, "")
	fs.DurationVar(&a.AuthenticateRateLimiterDuration, "authenticate-rate-limiter-duration", a.AuthenticateRateLimiterDuration, "")
	fs.BoolVar(&a.MultipleLogin, "multiple-login", a.MultipleLogin, "Allow multiple login with the same account, disable means only one user can login at the same time.")
	fs.StringVar(&a.JwtSecret, "jwt-secret", a.JwtSecret, "Secret to sign jwt token, must not be empty.")
	fs.DurationVar(&a.LoginHistoryRetentionPeriod, "login-history-retention-period", a.LoginHistoryRetentionPeriod, "login-history-retention-period defines how long login history should be kept.")
	fs.IntVar(&a.LoginHistoryMaximumEntries, "login-history-maximum-entries", a.LoginHistoryMaximumEntries, "login-history-maximum-entries defines how many entries of login history should be kept.")
	fs.DurationVar(&a.OAuthOptions.AccessTokenMaxAge, "access-token-max-age", a.OAuthOptions.AccessTokenMaxAge, "access-token-max-age control the lifetime of access tokens, 0 means no expiration.")
	fs.DurationVar(&a.MaximumClockSkew, "maximum-clock-skew", a.MaximumClockSkew, "The maximum time difference between the system clocks of the ks-apiserver that issued a JWT and the ks-apiserver that verified the JWT.")
}
