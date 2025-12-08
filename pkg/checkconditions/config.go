package checkconditions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	yamlv2 "gopkg.in/yaml.v2"
	"sigs.k8s.io/yaml"
)

const defaultConfigRelPath = ".config/check-conditions/check-conditions.yaml"

var ErrInvalidConfigYAML = errors.New("invalid config yaml")

// ConfigPath walks upwards from the working directory looking for the config file.
// It stops when it reaches / or /home without checking those directories.
func ConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to determine working directory: %w", err)
	}
	for {
		candidate := filepath.Join(dir, defaultConfigRelPath)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("checking config at %s: %w", candidate, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" || parent == filepath.Join("/", "home") {
			break
		}
		dir = parent
	}
	return "", nil
}

// Config holds the user provided resource-level overrides that should be applied
// when checking conditions.
type Config struct {
	Resources []ResourceConfig `yaml:"resources" json:"resources"`

	groupTree map[string]*groupNode
	path      string
}

// ResourceConfig describes how a single resource (group + name) should be treated.
type ResourceConfig struct {
	ResourceGroup string             `yaml:"resourceGroup,omitempty" json:"resourceGroup,omitempty"`
	Name          string             `yaml:"name" json:"name"`
	SkipIfTrue    []ConditionMatcher `yaml:"skipIfTrue,omitempty" json:"skipIfTrue,omitempty"`
	SkipIfFalse   []ConditionMatcher `yaml:"skipIfFalse,omitempty" json:"skipIfFalse,omitempty"`
}

func (r *ResourceConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var payload resourceConfigPayload
	if err := unmarshal(&payload); err != nil {
		return err
	}
	return r.decodePayload(&payload)
}

func (r *ResourceConfig) UnmarshalJSON(data []byte) error {
	var payload resourceConfigPayload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return err
	}
	return r.decodePayload(&payload)
}

// ConditionMatcher filters conditions by type/status/reason/message.
type ConditionMatcher struct {
	Type    string `yaml:"type" json:"type"`
	Status  string `yaml:"status" json:"status"`
	Reason  string `yaml:"reason" json:"reason"`
	Message string `yaml:"message" json:"message"`
}

type resourceConfigPayload struct {
	ResourceGroup string             `json:"resourceGroup" yaml:"resourceGroup"`
	Group         string             `json:"group" yaml:"group"`
	Name          string             `json:"name" yaml:"name"`
	SkipIfTrue    []ConditionMatcher `json:"skipIfTrue" yaml:"skipIfTrue"`
	SkipIfFalse   []ConditionMatcher `json:"skipIfFalse" yaml:"skipIfFalse"`
}

func (r *ResourceConfig) decodePayload(payload *resourceConfigPayload) error {
	r.Name = payload.Name
	if payload.ResourceGroup != "" {
		r.ResourceGroup = payload.ResourceGroup
	} else {
		r.ResourceGroup = payload.Group
	}
	if err := appendAndValidateMatchers(&r.SkipIfTrue, payload.SkipIfTrue, r.Name, "skipIfTrue"); err != nil {
		return err
	}
	if err := appendAndValidateMatchers(&r.SkipIfFalse, payload.SkipIfFalse, r.Name, "skipIfFalse"); err != nil {
		return err
	}
	return nil
}

func appendAndValidateMatchers(target *[]ConditionMatcher, matchers []ConditionMatcher, resourceName, field string) error {
	for _, matcher := range matchers {
		if strings.TrimSpace(matcher.Status) != "" {
			return fmt.Errorf("%w: resource %q %s entry must not set status", ErrInvalidConfigYAML, resourceName, field)
		}
		*target = append(*target, matcher)
	}
	return nil
}

func matcherExists(matchers []ConditionMatcher, candidate ConditionMatcher) bool {
	for _, matcher := range matchers {
		if matcher.Type == candidate.Type &&
			matcher.Reason == candidate.Reason &&
			matcher.Message == candidate.Message {
			return true
		}
	}
	return false
}

type skipTarget uint8

const (
	skipTargetTrue skipTarget = 1 << iota
	skipTargetFalse
	skipTargetBoth = skipTargetTrue | skipTargetFalse
)

func determineSkipTarget(status string) skipTarget {
	s := strings.TrimSpace(status)
	if s == "" || s == "*" {
		return skipTargetBoth
	}
	if strings.EqualFold(s, "true") {
		return skipTargetTrue
	}
	if strings.EqualFold(s, "false") {
		return skipTargetFalse
	}
	return skipTargetBoth
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

func canonicalGroup(group string) string {
	g := strings.TrimSpace(group)
	if strings.EqualFold(g, "core") {
		return ""
	}
	return g
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
	groupKey := group
	if strings.TrimSpace(groupKey) == "" {
		groupKey = "core"
	}
	res := c.ensureResourceConfig(groupKey, resource)
	statusPattern := wildcardOr(cStatus)
	entry := ConditionMatcher{
		Type: cType,
	}
	if cReason != "" && cReason != "*" {
		entry.Reason = cReason
	}
	if cMessage != "" && cMessage != "*" {
		entry.Message = cMessage
	}
	target := determineSkipTarget(cStatus)
	added := false
	if (target == skipTargetTrue || target == skipTargetBoth) && !matcherExists(res.SkipIfTrue, entry) {
		res.SkipIfTrue = append(res.SkipIfTrue, entry)
		added = true
	}
	if (target == skipTargetFalse || target == skipTargetBoth) && !matcherExists(res.SkipIfFalse, entry) {
		res.SkipIfFalse = append(res.SkipIfFalse, entry)
		added = true
	}
	if !added {
		return false, nil
	}
	if err := c.AddRegex(groupKey, resource, cType, statusPattern, wildcardOr(cReason), wildcardOr(cMessage)); err != nil {
		return false, err
	}
	if err := c.save(); err != nil {
		return true, err
	}
	return true, nil
}

func (c *Config) ensureResourceConfig(group, resource string) *ResourceConfig {
	for i := range c.Resources {
		if c.Resources[i].Name == resource {
			if c.Resources[i].ResourceGroup == "" && group != "" {
				c.Resources[i].ResourceGroup = group
			}
			return &c.Resources[i]
		}
		if c.Resources[i].ResourceGroup == group && c.Resources[i].Name == resource {
			return &c.Resources[i]
		}
	}
	c.Resources = append(c.Resources, ResourceConfig{
		ResourceGroup: group,
		Name:          resource,
	})
	return &c.Resources[len(c.Resources)-1]
}

func (c *Config) buildTree() error {
	for _, resource := range c.Resources {
		if strings.TrimSpace(resource.ResourceGroup) == "" {
			return fmt.Errorf("%w: resource %q missing group", ErrInvalidConfigYAML, resource.Name)
		}
		group := canonicalGroup(resource.ResourceGroup)
		name := wildcardOr(resource.Name)

		if err := c.addMatchers(group, name, resource.SkipIfTrue, "True"); err != nil {
			return err
		}
		if err := c.addMatchers(group, name, resource.SkipIfFalse, "False"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) addMatchers(group, name string, matchers []ConditionMatcher, defaultStatus string) error {
	for _, matcher := range matchers {
		if matcher.Type == "" {
			return fmt.Errorf("%w: resource %q ignore entry missing type", ErrInvalidConfigYAML, name)
		}
		status := matcher.Status
		if strings.TrimSpace(status) == "" {
			status = defaultStatus
		}
		if err := c.AddRegex(group, name, matcher.Type, wildcardOr(status),
			wildcardOr(matcher.Reason), wildcardOr(matcher.Message)); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidConfigYAML, err)
		}
	}
	return nil
}

// LoadConfig finds the config path and loads the file if it exists.
func LoadConfig() (*Config, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, "", err
	}
	if path == "" {
		return nil, "", nil
	}
	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		return nil, "", err
	}
	return cfg, path, nil
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
		return nil, fmt.Errorf("%w: unable to parse config at %s: %w", ErrInvalidConfigYAML, path, err)
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
	var resourcesSlice []yamlv2.MapSlice
	for _, resource := range c.Resources {
		resourceItem := yamlv2.MapSlice{
			{Key: "name", Value: resource.Name},
		}
		if resource.ResourceGroup != "" {
			resourceItem = append(resourceItem, yamlv2.MapItem{Key: "resourceGroup", Value: resource.ResourceGroup})
		}
		if matchers := marshalMatchers(resource.SkipIfTrue); len(matchers) > 0 {
			resourceItem = append(resourceItem, yamlv2.MapItem{Key: "skipIfTrue", Value: matchers})
		}
		if matchers := marshalMatchers(resource.SkipIfFalse); len(matchers) > 0 {
			resourceItem = append(resourceItem, yamlv2.MapItem{Key: "skipIfFalse", Value: matchers})
		}
		resourcesSlice = append(resourcesSlice, resourceItem)
	}
	data, err := yamlv2.Marshal(yamlv2.MapSlice{
		{Key: "resources", Value: resourcesSlice},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}

func marshalMatchers(matchers []ConditionMatcher) []yamlv2.MapSlice {
	var result []yamlv2.MapSlice
	for _, matcher := range matchers {
		item := yamlv2.MapSlice{
			{Key: "type", Value: matcher.Type},
		}
		if strings.TrimSpace(matcher.Reason) != "" {
			item = append(item, yamlv2.MapItem{Key: "reason", Value: matcher.Reason})
		}
		result = append(result, item)
	}
	return result
}
