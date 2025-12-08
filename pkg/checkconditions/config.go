package checkconditions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

const defaultConfigRelPath = ".config/check-conditions/check-conditions.yaml"

// ConfigPath returns the path to the default config file in the user's home.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to find home directory: %w", err)
	}
	return filepath.Join(home, defaultConfigRelPath), nil
}

// Config holds the user provided resource-level overrides that should be applied
// when checking conditions.
type Config struct {
	Resources []ResourceConfig `yaml:"resources"`

	groupTree map[string]*groupNode
	path      string
}

// ResourceConfig describes how a single resource (group + name) should be treated.
type ResourceConfig struct {
	Group  string             `yaml:"group"`
	Name   string             `yaml:"name"`
	Ignore []ConditionMatcher `yaml:"ignore"`
}

// ConditionMatcher filters conditions by type/status/reason/message.
type ConditionMatcher struct {
	Type    string `yaml:"type"`
	Status  string `yaml:"status"`
	Reason  string `yaml:"reason"`
	Message string `yaml:"message"`
}

// AddRegex stores the provided tuple in the in-memory tree so skipConditionViaConfig
// can efficiently match on each attribute. Each provided value is converted to a
// regex (wildcards: "*" -> ".*") before being inserted.
func (c *Config) AddRegex(group, resource, cType, cStatus, cReason, cMessage string) error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.groupTree == nil {
		c.groupTree = make(map[string]*groupNode)
	}

	groupNodeVal := c.groupTree[group]
	if groupNodeVal == nil {
		re, err := compilePattern(group)
		if err != nil {
			return fmt.Errorf("group pattern %q: %w", group, err)
		}
		groupNodeVal = &groupNode{
			regex:     re,
			resources: make(map[string]*resourceNode),
		}
		c.groupTree[group] = groupNodeVal
	}

	resNode := groupNodeVal.resources[resource]
	if resNode == nil {
		re, err := compilePattern(resource)
		if err != nil {
			return fmt.Errorf("resource pattern %q: %w", resource, err)
		}
		resNode = &resourceNode{
			regex: re,
			types: make(map[string]*typeNode),
		}
		groupNodeVal.resources[resource] = resNode
	}

	typeNodeVal := resNode.types[cType]
	if typeNodeVal == nil {
		re, err := compilePattern(cType)
		if err != nil {
			return fmt.Errorf("type pattern %q: %w", cType, err)
		}
		typeNodeVal = &typeNode{
			regex:    re,
			statuses: make(map[string]*statusNode),
		}
		resNode.types[cType] = typeNodeVal
	}

	statusNodeVal := typeNodeVal.statuses[cStatus]
	if statusNodeVal == nil {
		re, err := compilePattern(cStatus)
		if err != nil {
			return fmt.Errorf("status pattern %q: %w", cStatus, err)
		}
		statusNodeVal = &statusNode{
			regex:   re,
			reasons: make(map[string]*reasonNode),
		}
		typeNodeVal.statuses[cStatus] = statusNodeVal
	}

	reasonNodeVal := statusNodeVal.reasons[cReason]
	if reasonNodeVal == nil {
		re, err := compilePattern(cReason)
		if err != nil {
			return fmt.Errorf("reason pattern %q: %w", cReason, err)
		}
		reasonNodeVal = &reasonNode{
			regex:    re,
			messages: make(map[string]*messageNode),
		}
		statusNodeVal.reasons[cReason] = reasonNodeVal
	}

	messageNodeVal := reasonNodeVal.messages[cMessage]
	if messageNodeVal == nil {
		re, err := compilePattern(cMessage)
		if err != nil {
			return fmt.Errorf("message pattern %q: %w", cMessage, err)
		}
		messageNodeVal = &messageNode{regex: re}
		reasonNodeVal.messages[cMessage] = messageNodeVal
	}

	return nil
}

func compilePattern(value string) (*regexp.Regexp, error) {
	escaped := regexp.QuoteMeta(value)
	pattern := strings.ReplaceAll(escaped, "\\*", ".*")
	return regexp.Compile("^" + pattern + "$")
}

func wildcardOr(value string) string {
	if value == "" {
		return "*"
	}
	return value
}

type groupNode struct {
	regex     *regexp.Regexp
	resources map[string]*resourceNode
}

type resourceNode struct {
	regex *regexp.Regexp
	types map[string]*typeNode
}

type typeNode struct {
	regex    *regexp.Regexp
	statuses map[string]*statusNode
}

type statusNode struct {
	regex   *regexp.Regexp
	reasons map[string]*reasonNode
}

type reasonNode struct {
	regex    *regexp.Regexp
	messages map[string]*messageNode
}

type messageNode struct {
	regex *regexp.Regexp
}

func (c *Config) shouldSkip(group, resource, cType, cStatus, cReason, cMessage string) bool {
	if c == nil || c.groupTree == nil {
		return false
	}
	for _, node := range c.groupTree {
		if node == nil || node.regex == nil || !node.regex.MatchString(group) {
			continue
		}
		if matchResources(node.resources, resource, cType, cStatus, cReason, cMessage) {
			return true
		}
	}
	return false
}

func matchResources(resources map[string]*resourceNode, resource, cType, cStatus, cReason, cMessage string) bool {
	for _, node := range resources {
		if node == nil || node.regex == nil || !node.regex.MatchString(resource) {
			continue
		}
		if matchTypes(node.types, cType, cStatus, cReason, cMessage) {
			return true
		}
	}
	return false
}

func matchTypes(types map[string]*typeNode, cType, cStatus, cReason, cMessage string) bool {
	for _, node := range types {
		if node == nil || node.regex == nil || !node.regex.MatchString(cType) {
			continue
		}
		if matchStatuses(node.statuses, cStatus, cReason, cMessage) {
			return true
		}
	}
	return false
}

func matchStatuses(statuses map[string]*statusNode, cStatus, cReason, cMessage string) bool {
	for _, node := range statuses {
		if node == nil || node.regex == nil || !node.regex.MatchString(cStatus) {
			continue
		}
		if matchReasons(node.reasons, cReason, cMessage) {
			return true
		}
	}
	return false
}

func matchReasons(reasons map[string]*reasonNode, cReason, cMessage string) bool {
	for _, node := range reasons {
		if node == nil || node.regex == nil || !node.regex.MatchString(cReason) {
			continue
		}
		if matchMessages(node.messages, cMessage) {
			return true
		}
	}
	return false
}

func matchMessages(messages map[string]*messageNode, cMessage string) bool {
	for _, node := range messages {
		if node == nil || node.regex == nil {
			continue
		}
		if node.regex.MatchString(cMessage) {
			return true
		}
	}
	return false
}

func (c *Config) addLegacyIgnore(group, resource, cType, cStatus, cReason, cMessage string) (bool, error) {
	if c == nil {
		return false, errors.New("config is nil")
	}
	if c.path == "" {
		return false, errors.New("config path not set")
	}
	res := c.ensureResourceConfig(group, resource)
	statusPattern := wildcardOr(cStatus)
	entry := ConditionMatcher{
		Type:   cType,
		Status: cStatus,
	}
	if cReason != "" && cReason != "*" {
		entry.Reason = cReason
	}
	if cMessage != "" && cMessage != "*" {
		entry.Message = cMessage
	}
	for _, existing := range res.Ignore {
		if existing.Type == entry.Type && existing.Status == entry.Status &&
			existing.Reason == entry.Reason && existing.Message == entry.Message {
			return false, nil
		}
	}
	res.Ignore = append(res.Ignore, entry)
	if err := c.AddRegex(group, resource, cType, statusPattern, wildcardOr(cReason), wildcardOr(cMessage)); err != nil {
		return false, err
	}
	if err := c.save(); err != nil {
		return true, err
	}
	return true, nil
}

func (c *Config) ensureResourceConfig(group, resource string) *ResourceConfig {
	for i := range c.Resources {
		if c.Resources[i].Group == group && c.Resources[i].Name == resource {
			return &c.Resources[i]
		}
	}
	c.Resources = append(c.Resources, ResourceConfig{
		Group: group,
		Name:  resource,
	})
	return &c.Resources[len(c.Resources)-1]
}

func (c *Config) buildTree() error {
	for _, resource := range c.Resources {
		group := wildcardOr(resource.Group)
		name := wildcardOr(resource.Name)

		for _, matcher := range resource.Ignore {
			if matcher.Type == "" {
				return fmt.Errorf("resource %q ignore entry missing type", resource.Name)
			}
			status := wildcardOr(matcher.Status)
			if err := c.AddRegex(group, name, matcher.Type, status,
				wildcardOr(matcher.Reason), wildcardOr(matcher.Message)); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadConfig reads the config from the default path (~/.config/check-conditions/check-conditions.yaml).
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadConfigFromPath(path)
}

// LoadConfigFromPath reads and parses the config stored at the provided path. If the file
// does not exist, nil is returned without an error.
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to read config at %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to parse config at %s: %w", path, err)
	}
	cfg.path = path
	if err := cfg.buildTree(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SetPath registers the path used when loading or creating this config.
func (c *Config) SetPath(path string) {
	c.path = path
}

func (c *Config) save() error {
	if c.path == "" {
		return errors.New("config path not set")
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}
