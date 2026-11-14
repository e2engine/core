package model

import (
	"encoding/json"
	nativeerrors "errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ygrebnov/errorc"
	modellib "github.com/ygrebnov/model"
	"github.com/ygrebnov/model/validation"

	"github.com/e2engine/core/model/matcher"
	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

var (
	rules    []modellib.Rule
	newRules sync.Once
)

func GetValidationRules() []modellib.Rule {
	newRules.Do(func() {
		rules = []modellib.Rule{
			getIDRule(),
			getNetworkAddressRule(),
			getHTTPURLRule(),
			getValidJSONRule(),
			matcher.GetStringMatcherRule(),
			matcher.GetStringMatchersRule(),
			getStringMatcherValidJSONRule(),
			matcher.GetGRPCMessageMatcherRule(),
			getIPRule(),
		}
	})

	return rules
}

var (
	environmentBinding *modellib.Binding[Environment]
	testBinding        *modellib.Binding[Test]
	testSuiteBinding   *modellib.Binding[TestSuite]

	newEnvironmentBinding sync.Once
	newTestBinding        sync.Once
	newTestSuiteBinding   sync.Once

	environmentBindingErr error
	testBindingErr        error
	testSuiteBindingErr   error
)

func getEnvironmentBinding() (*modellib.Binding[Environment], error) {
	newEnvironmentBinding.Do(func() {
		environmentBinding, environmentBindingErr =
			modellib.NewBinding[Environment](modellib.WithRules(GetValidationRules()...))
	})
	return environmentBinding, environmentBindingErr
}

func getTestBinding() (*modellib.Binding[Test], error) {
	newTestBinding.Do(func() {
		testBinding, testBindingErr = modellib.NewBinding[Test](modellib.WithRules(GetValidationRules()...))
	})
	return testBinding, testBindingErr
}

func getTestSuiteBinding() (*modellib.Binding[TestSuite], error) {
	newTestSuiteBinding.Do(func() {
		testSuiteBinding, testSuiteBindingErr =
			modellib.NewBinding[TestSuite](modellib.WithRules(GetValidationRules()...))
	})
	return testSuiteBinding, testSuiteBindingErr
}

func getIDRule() modellib.Rule {
	return modellib.NewRule("id", func(v string, _ ...string) error {
		r := regexp.MustCompile("^[a-z0-9]{64}$")
		if !r.MatchString(v) {
			return errorc.New("value must contain exactly 64 lowercase alphanumeric characters")
		}

		return nil
	})
}

func getNetworkAddressRule() modellib.Rule {
	return modellib.NewRule[string]("networkaddress", func(address string, _ ...string) error {
		return validateNetworkAddress(address)
	})
}

func validateNetworkAddress(address string) error {
	if address == "" {
		return errorc.New("network address must not be empty")
	}

	host, portRaw, err := net.SplitHostPort(address)
	if err != nil {
		return errorc.New("network address must be in host:port form")
	}

	if strings.TrimSpace(host) == "" {
		return errorc.New("network address host must not be empty")
	}

	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil || port == 0 {
		return errorc.New("network address port must be an integer between 1 and 65535")
	}

	return nil
}

func getHTTPURLRule() modellib.Rule {
	return modellib.NewRule[string](
		"httpurl",
		func(value string, _ ...string) error {
			return validateHTTPURL(value)
		},
	)
}

func validateHTTPURL(value string) error {
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return errorc.New("target must be a valid HTTP URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errorc.New("target URL scheme must be http or https")
	}

	if u.Hostname() == "" {
		return errorc.New("target URL host must not be empty")
	}

	if u.User != nil {
		return errorc.New("target URL must not contain user info")
	}

	if u.RawQuery != "" || u.Fragment != "" {
		return errorc.New("target URL must not contain query parameters or fragment")
	}

	if portRaw := u.Port(); portRaw != "" {
		port, err := strconv.ParseUint(portRaw, 10, 16)
		if err != nil || port == 0 {
			return errorc.New("target URL port must be an integer between 1 and 65535")
		}
	}

	return nil
}

func getValidJSONRule() modellib.Rule {
	return modellib.NewRule[string]("validjson", func(value string, _ ...string) error {
		if !json.Valid([]byte(value)) {
			return errorc.New("value must contain valid JSON")
		}

		return nil
	})
}

func getIPRule() modellib.Rule {
	return modellib.NewRule[string]("ip", func(ip string, _ ...string) error {
		if net.ParseIP(ip) == nil {
			return errorc.New("invalid E2Engine listen interface IP address")
		}
		return nil
	})
}

func handleValidationError(err error, specKind EntityKind) error {
	var fields string

	if validationErr, ok := nativeerrors.AsType[*validation.Error](err); ok {
		fields = strings.Join(validationErr.Fields(), ", ")

		validationErrs := make([]string, 0, validationErr.Len())
		for _, path := range validationErr.Fields() {
			for _, fieldErr := range validationErr.ForField(path) {
				validationErrs = append(validationErrs, fieldErr.Err.Error())
			}
		}
		combinedErr := strings.Join(validationErrs, ", ")

		return errorc.With(
			errors.ErrInvalidSpec,
			errorc.String(keys.SpecKind, string(specKind)),
			errorc.String(keys.Fields, fields),
			errorc.String(keys.Validation, combinedErr),
		)
	}

	return errorc.With(
		errors.ErrInvalidSpec,
		errorc.String(keys.SpecKind, string(specKind)),
		errorc.String(keys.Validation, err.Error()),
	)
}
