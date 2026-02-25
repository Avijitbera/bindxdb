package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"time"
)

type RequiredValidator struct {
	Key string
}

func (v *RequiredValidator) Validate(key string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("%s is required", key)
	}
	if str, ok := value.(string); ok && str == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}
	return nil
}

type TypeValidator struct {
	ExpectedType reflect.Kind
}

func (v *TypeValidator) Validate(key string, value interface{}) error {
	actualType := reflect.TypeOf(value)
	if actualType == nil {
		return fmt.Errorf("%s: value is nil", key)
	}

	if actualType.Kind() != v.ExpectedType {
		return fmt.Errorf("%s: expected type %s, got %s",
			key, v.ExpectedType, actualType.Kind())
	}
	return nil
}

type RangeValidator struct {
	Min float64
	Max float64
}

func (v *RangeValidator) Validate(key string, value interface{}) error {
	var num float64

	switch val := value.(type) {
	case int:
		num = float64(val)
	case int64:
		num = float64(val)
	case float64:
		num = val
	case json.Number:
		f, err := val.Float64()
		if err != nil {
			return fmt.Errorf("%s: cannot convert to number: %w", key, err)
		}
		num = f
	default:
		return fmt.Errorf("%s: expected type int, int64, float64 or json.Number, got %T", key, value)
	}
	if num < v.Min || num > v.Max {
		return fmt.Errorf("%s: value out of range [%.2f, %.2f]", key, v.Min, v.Max)
	}
	return nil
}

type PatternValidator struct {
	Pattern string
	regex   *regexp.Regexp
}

func NewPatternValidator(pattern string) (*PatternValidator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}

	return &PatternValidator{
		Pattern: pattern,
		regex:   regex,
	}, nil
}

func (v *PatternValidator) Validate(key string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected string for pattern validation", key)
	}

	if !v.regex.MatchString(str) {
		return fmt.Errorf("%s: value %q does not match pattern %s", key, str, v.Pattern)
	}
	return nil
}

type EnumValidator struct {
	Allowed []interface{}
}

func (v *EnumValidator) Validate(key string, value interface{}) error {
	for _, allowed := range v.Allowed {
		if reflect.DeepEqual(allowed, value) {
			return nil
		}
	}
	return fmt.Errorf("%s: value %v not in allowed set %v", key, value, v.Allowed)
}

type DurationValidator struct {
	Min time.Duration
	Max time.Duration
}

func (v *DurationValidator) Validate(key string, value interface{}) error {
	var d time.Duration
	switch val := value.(type) {
	case string:
		parsed, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("%s: invalid duration string %q: %w", key, val, err)
		}
		d = parsed
	case int:
		d = time.Duration(val)
	case float32:
		d = time.Duration(val)
	default:
		return fmt.Errorf("%s: expected duration, got %T", key, value)
	}
	if d < v.Min || (v.Max > 0 && d > v.Max) {
		return fmt.Errorf("%s: duration %v out of range [%v, %v]", key, d, v.Min, v.Max)
	}

	return nil
}

type FileValidator struct {
	MustExist   bool
	MustBeDir   bool
	MustBeFile  bool
	Permissions os.FileMode
}

func (v *FileValidator) Validate(key string, value interface{}) error {
	path, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected file path string", key)
	}

	if v.MustExist {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: file %s does not exist", key, path)
		}
		if err != nil {
			return fmt.Errorf("%s: error accessing %s: %w", key, path, err)
		}

		if v.MustBeFile && info.IsDir() {
			return fmt.Errorf("%s: %s is not a file", key, path)
		}
		if v.Permissions != 0 {
			if info.Mode().Perm() != v.Permissions {
				return fmt.Errorf("%s: %s has permissions %o, expected %o",
					key, path, info.Mode().Perm(), v.Permissions)
			}
		}
	}

	return nil
}

type URLValidator struct {
	Schemas []string
}

func (v *URLValidator) Validate(key string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected URL string", key)
	}
	u, err := url.Parse(str)
	if err != nil {
		return fmt.Errorf("%s: invalid URL %q: %w", key, str, err)
	}

	if len(v.Schemas) > 0 {
		validScheme := false
		for _, scheme := range v.Schemas {
			if u.Scheme == scheme {
				validScheme = true
				break
			}
		}
		if !validScheme {
			return fmt.Errorf("%s: URL schema %q not allowed (allowed: %v)",
				key, u.Scheme, v.Schemas)
		}
	}
	return nil

}

type IPValidator struct {
	AllowIPv4 bool
	AllowIPv6 bool
	CIDRs     []*net.IPNet
}

func NewIPValidator() *IPValidator {
	return &IPValidator{
		AllowIPv4: true,
		AllowIPv6: true,
	}
}

func (v *IPValidator) AddCIDR(cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err

	}

	v.CIDRs = append(v.CIDRs, ipNet)
	return nil
}

func (v *IPValidator) Validate(key string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected IP address string", key)
	}

	ip := net.ParseIP(str)
	if ip == nil {
		return fmt.Errorf("%s: invalid IP address %q", key, str)
	}

	if ip.To4() != nil {
		if !v.AllowIPv4 {
			return fmt.Errorf("%s: IPv4 addresses not allowed", key)
		}
	} else {
		if !v.AllowIPv6 {
			return fmt.Errorf("%s: IPv6 addresses not allowed", key)
		}
	}

	if len(v.CIDRs) > 0 {
		allowed := false
		for _, cidr := range v.CIDRs {
			if cidr.Contains(ip) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%s: IP %q not in allowed CIDR ranges", key, str)
		}
	}
	return nil
}
