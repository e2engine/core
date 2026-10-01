package model

import (
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

var validationRules []modellib.Rule

func init() {
	validationRules = make([]modellib.Rule, 4)

	idRule, err := getIDRule()
	if err != nil {
		panic(err)
	}

	networkAddressRule, err := getNetworkAddressRule()
	if err != nil {
		panic(err)
	}

	httpURLRule, err := getHTTPURLRule()
	if err != nil {
		panic(err)
	}

	validJSONRule, err := getValidJSONRule()
	if err != nil {
		panic(err)
	}

	validationRules[0] = idRule
	validationRules[1] = networkAddressRule
	validationRules[2] = httpURLRule
	validationRules[3] = validJSONRule

	environmentBinding, err = modellib.NewBinding[Environment](modellib.WithRules(validationRules...))
	if err != nil {
		panic(err)
	}

	testBinding, err = modellib.NewBinding[Test](modellib.WithRules(validationRules...))
	if err != nil {
		panic(err)
	}

	testSuiteBinding, err = modellib.NewBinding[TestSuite](modellib.WithRules(validationRules...))
	if err != nil {
		panic(err)
	}
}

func GetValidationRules() []modellib.Rule {
	return slices.Clone(validationRules)
}

func getIDRule() (modellib.Rule, error) {
	return modellib.NewRule("id", func(v string, _ ...string) error {
		r := regexp.MustCompile("^[a-z0-9]{64}$")
		if !r.MatchString(v) {
			return errorc.New("value must contain exactly 64 lowercase alphanumeric characters")
		}

		return nil
	})
}

func getNetworkAddressRule() (modellib.Rule, error) {
	return modellib.NewRule[string]("networkaddress", func(address string, _ ...string) error {
		if address == "" {
			return errorc.New("network address must not be empty")
		}

		host, portRaw, err := net.SplitHostPort(address)
		if err != nil {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.Validation, "network address must be in host:port form"),
				errorc.Error(keys.Cause, err),
			)
		}

		if strings.TrimSpace(host) == "" {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(keys.Validation, "network address host must not be empty"),
			)
		}

		port, err := strconv.ParseUint(portRaw, 10, 16)
		if err != nil || port == 0 {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(
					keys.Validation,
					"network address port must be an integer between 1 and 65535",
				),
			)
		}

		return nil
	})
}

func getHTTPURLRule() (modellib.Rule, error) {
	return modellib.NewRule[string](
		"httpurl",
		func(value string, _ ...string) error {
			u, err := url.ParseRequestURI(value)
			if err != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"target must be a valid HTTP URL",
					),
					errorc.Error(keys.Cause, err),
				)
			}

			if u.Scheme != "http" &&
				u.Scheme != "https" {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"target URL scheme must be http or https",
					),
				)
			}

			if u.Host == "" {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"target URL host must not be empty",
					),
				)
			}

			if u.User != nil {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"target URL must not contain user info",
					),
				)
			}

			if u.RawQuery != "" || u.Fragment != "" {
				return errorc.With(
					errors.ErrInvalidSpec,
					errorc.String(
						keys.Validation,
						"target URL must not contain query parameters or fragment",
					),
				)
			}

			if portRaw := u.Port(); portRaw != "" {
				port, err := strconv.ParseUint(
					portRaw,
					10,
					16,
				)
				if err != nil || port == 0 {
					return errorc.With(
						errors.ErrInvalidSpec,
						errorc.String(
							keys.Validation,
							"target URL port must be an integer between 1 and 65535",
						),
					)
				}
			}

			return nil
		},
	)
}

func getValidJSONRule() (modellib.Rule, error) {
	return modellib.NewRule[string]("validjson", func(value string, _ ...string) error {
		if !json.Valid([]byte(value)) {
			return errorc.With(
				errors.ErrInvalidSpec,
				errorc.String(
					keys.Validation,
					"value must contain valid JSON",
				),
			)
		}

		return nil
	})
}
