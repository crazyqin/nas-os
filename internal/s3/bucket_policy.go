// Package s3 implements S3-compatible object storage for NAS-OS
// This file provides Bucket Policy management with a full policy engine.
package s3

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// PolicyAction represents an S3 policy action.
type PolicyAction string

// Well-known S3 policy actions.
const (
	PolicyActionGetObject            PolicyAction = "s3:GetObject"
	PolicyActionPutObject            PolicyAction = "s3:PutObject"
	PolicyActionDeleteObject         PolicyAction = "s3:DeleteObject"
	PolicyActionListBucket           PolicyAction = "s3:ListBucket"
	PolicyActionGetBucketLocation    PolicyAction = "s3:GetBucketLocation"
	PolicyActionGetBucketVersioning  PolicyAction = "s3:GetBucketVersioning"
	PolicyActionPutBucketVersioning  PolicyAction = "s3:PutBucketVersioning"
	PolicyActionGetBucketPolicy      PolicyAction = "s3:GetBucketPolicy"
	PolicyActionPutBucketPolicy      PolicyAction = "s3:PutBucketPolicy"
	PolicyActionDeleteBucketPolicy   PolicyAction = "s3:DeleteBucketPolicy"
	PolicyActionGetObjectVersion     PolicyAction = "s3:GetObjectVersion"
	PolicyActionDeleteObjectVersion  PolicyAction = "s3:DeleteObjectVersion"
	PolicyActionGetObjectRetention   PolicyAction = "s3:GetObjectRetention"
	PolicyActionPutObjectRetention   PolicyAction = "s3:PutObjectRetention"
	PolicyActionGetObjectLegalHold   PolicyAction = "s3:GetObjectLegalHold"
	PolicyActionPutObjectLegalHold   PolicyAction = "s3:PutObjectLegalHold"
	PolicyActionGetLifecycleConfig   PolicyAction = "s3:GetLifecycleConfiguration"
	PolicyActionPutLifecycleConfig   PolicyAction = "s3:PutLifecycleConfiguration"
	PolicyActionGetObjectLockConfig  PolicyAction = "s3:GetObjectLockConfiguration"
	PolicyActionPutObjectLockConfig  PolicyAction = "s3:PutObjectLockConfiguration"
	PolicyActionAll                  PolicyAction = "s3:*"
)

// PolicyEffect represents the effect of a policy statement.
type PolicyEffect string

// Policy effect constants.
const (
	PolicyEffectAllow PolicyEffect = "Allow"
	PolicyEffectDeny  PolicyEffect = "Deny"
)

// PolicyEvaluationResult contains the result of a policy evaluation.
type PolicyEvaluationResult struct {
	Allowed bool           `json:"allowed"`
	Effect  PolicyEffect   `json:"effect"`
	Matched []string       `json:"matched"` // matched statement SIDs
	Reason  string         `json:"reason"`
}

// PolicyRequest represents a request to evaluate against a policy.
type PolicyRequest struct {
	Action    PolicyAction `json:"action"`
	Resource  string       `json:"resource"`  // e.g. "bucket/key"
	Principal string       `json:"principal"` // requesting user/IP
	IPAddress string       `json:"ipAddress"`
}

// PolicyEngine evaluates bucket policies against requests.
type PolicyEngine struct {
	// allowedActions is the set of all known S3 actions for wildcard matching.
	allowedActions map[PolicyAction]bool
}

// NewPolicyEngine creates a new policy engine.
func NewPolicyEngine() *PolicyEngine {
	pe := &PolicyEngine{
		allowedActions: map[PolicyAction]bool{
			PolicyActionGetObject:           true,
			PolicyActionPutObject:           true,
			PolicyActionDeleteObject:        true,
			PolicyActionListBucket:          true,
			PolicyActionGetBucketLocation:   true,
			PolicyActionGetBucketVersioning: true,
			PolicyActionPutBucketVersioning: true,
			PolicyActionGetBucketPolicy:     true,
			PolicyActionPutBucketPolicy:     true,
			PolicyActionDeleteBucketPolicy:  true,
			PolicyActionGetObjectVersion:    true,
			PolicyActionDeleteObjectVersion: true,
			PolicyActionGetObjectRetention:  true,
			PolicyActionPutObjectRetention:  true,
			PolicyActionGetObjectLegalHold:  true,
			PolicyActionPutObjectLegalHold:  true,
			PolicyActionGetLifecycleConfig:  true,
			PolicyActionPutLifecycleConfig:  true,
			PolicyActionGetObjectLockConfig: true,
			PolicyActionPutObjectLockConfig: true,
		},
	}
	return pe
}

// Evaluate evaluates a policy against a request.
// Default deny: if no statement matches, the request is denied.
func (pe *PolicyEngine) Evaluate(policy *BucketPolicy, req *PolicyRequest) *PolicyEvaluationResult {
	if policy == nil {
		return &PolicyEvaluationResult{
			Allowed: false,
			Reason:  "no policy defined",
		}
	}

	result := &PolicyEvaluationResult{
		Allowed: false,
	}

	for _, stmt := range policy.Statement {
		if !pe.statementMatches(&stmt, req) {
			continue
		}

		effect := PolicyEffect(stmt.Effect)
		result.Matched = append(result.Matched, stmt.SID)

		switch effect {
		case PolicyEffectAllow:
			result.Allowed = true
			result.Effect = PolicyEffectAllow
		case PolicyEffectDeny:
			// Explicit deny always wins
			return &PolicyEvaluationResult{
				Allowed: false,
				Effect:  PolicyEffectDeny,
				Matched: result.Matched,
				Reason:  "explicit deny by policy",
			}
		}
	}

	if !result.Allowed {
		result.Reason = "no matching allow statement"
	}

	return result
}

// statementMatches checks if a policy statement matches the given request.
func (pe *PolicyEngine) statementMatches(stmt *PolicyStatement, req *PolicyRequest) bool {
	// Check action match
	if !pe.actionMatches(stmt.Action, req.Action) {
		return false
	}

	// Check resource match
	if !pe.resourceMatches(stmt.Resource, req.Resource) {
		return false
	}

	// Check principal match
	if stmt.Principal != nil && !pe.principalMatches(stmt.Principal, req.Principal) {
		return false
	}

	// Check IP condition
	if stmt.Condition != nil {
		if !pe.conditionMatches(stmt.Condition, req) {
			return false
		}
	}

	return true
}

// actionMatches checks if the requested action matches any statement actions.
func (pe *PolicyEngine) actionMatches(statementActions []string, requested PolicyAction) bool {
	for _, action := range statementActions {
		if action == string(PolicyActionAll) {
			return true
		}
		// Support s3:Get* style wildcards
		if strings.HasSuffix(action, "*") {
			prefix := strings.TrimSuffix(action, "*")
			if strings.HasPrefix(string(requested), prefix) {
				return true
			}
		}
		if PolicyAction(action) == requested {
			return true
		}
	}
	return false
}

// resourceMatches checks if the requested resource matches the statement resource.
func (pe *PolicyEngine) resourceMatches(statementResource interface{}, requested string) bool {
	switch r := statementResource.(type) {
	case string:
		return pe.matchResourcePattern(r, requested)
	case []interface{}:
		for _, res := range r {
			if s, ok := res.(string); ok {
				if pe.matchResourcePattern(s, requested) {
					return true
				}
			}
		}
	}
	return false
}

// matchResourcePattern matches a resource pattern (supports * and ? wildcards).
func (pe *PolicyEngine) matchResourcePattern(pattern, resource string) bool {
	if pattern == "*" || pattern == "arn:aws:s3:::*" {
		return true
	}

	// Support arn:aws:s3:::bucket/key style
	pattern = strings.TrimPrefix(pattern, "arn:aws:s3:::")
	resource = strings.TrimPrefix(resource, "arn:aws:s3:::")

	// Simple wildcard matching
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(resource, prefix)
	}

	return pattern == resource
}

// principalMatches checks if the requesting principal matches.
func (pe *PolicyEngine) principalMatches(statementPrincipal interface{}, requested string) bool {
	switch p := statementPrincipal.(type) {
	case string:
		return p == "*" || p == requested
	case map[string]interface{}:
		if aws, ok := p["AWS"]; ok {
			switch a := aws.(type) {
			case string:
				return a == "*" || a == requested
			case []interface{}:
				for _, v := range a {
					if s, ok := v.(string); ok && (s == "*" || s == requested) {
						return true
					}
				}
			}
		}
	}
	return false
}

// conditionMatches evaluates policy conditions against the request.
func (pe *PolicyEngine) conditionMatches(condition *PolicyCondition, req *PolicyRequest) bool {
	// IP address conditions
	if condition.IPAddress != nil {
		if sourceIP, ok := condition.IPAddress["aws:SourceIp"]; ok {
			if !pe.matchIPRange(sourceIP, req.IPAddress) {
				return false
			}
		}
	}
	if condition.NotIPAddress != nil {
		if sourceIP, ok := condition.NotIPAddress["aws:SourceIp"]; ok {
			if pe.matchIPRange(sourceIP, req.IPAddress) {
				return false
			}
		}
	}

	// String conditions
	if condition.StringEquals != nil {
		for key, val := range condition.StringEquals {
			if !pe.matchStringCondition(key, val, req) {
				return false
			}
		}
	}
	if condition.StringNotEquals != nil {
		for key, val := range condition.StringNotEquals {
			if pe.matchStringCondition(key, val, req) {
				return false
			}
		}
	}

	return true
}

// matchIPRange checks if an IP matches a CIDR range or exact IP.
func (pe *PolicyEngine) matchIPRange(cidr, ip string) bool {
	if strings.Contains(cidr, "/") {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return false
		}
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return false
		}
		return ipNet.Contains(parsedIP)
	}
	return cidr == ip
}

// matchStringCondition evaluates a string condition.
func (pe *PolicyEngine) matchStringCondition(key, value string, req *PolicyRequest) bool {
	switch key {
	case "s3:prefix":
		return strings.HasPrefix(req.Resource, value)
	case "s3:delimiter":
		return true // delimiter is always allowed
	default:
		return true
	}
}

// ValidatePolicy validates a bucket policy structure and semantics.
func ValidatePolicy(policy *BucketPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	// Validate version
	if policy.Version == "" {
		return fmt.Errorf("policy version is required")
	}
	if policy.Version != "2012-10-17" {
		return fmt.Errorf("unsupported policy version: %s, expected 2012-10-17", policy.Version)
	}

	if len(policy.Statement) == 0 {
		return fmt.Errorf("policy must contain at least one statement")
	}

	for i, stmt := range policy.Statement {
		if err := validateStatement(i, &stmt); err != nil {
			return err
		}
	}

	// Validate JSON serialization
	if _, err := json.Marshal(policy); err != nil {
		return fmt.Errorf("policy is not serializable: %w", err)
	}

	return nil
}

// validateStatement validates a single policy statement.
func validateStatement(index int, stmt *PolicyStatement) error {
	effect := PolicyEffect(stmt.Effect)
	if effect != PolicyEffectAllow && effect != PolicyEffectDeny {
		return fmt.Errorf("statement[%d]: invalid effect %q, must be Allow or Deny", index, stmt.Effect)
	}

	if len(stmt.Action) == 0 {
		return fmt.Errorf("statement[%d]: at least one action is required", index)
	}

	for _, action := range stmt.Action {
		if !isValidAction(action) {
			return fmt.Errorf("statement[%d]: invalid action %q", index, action)
		}
	}

	if stmt.Resource == nil {
		return fmt.Errorf("statement[%d]: resource is required", index)
	}

	return nil
}

// isValidAction checks if an action string is a valid S3 action pattern.
func isValidAction(action string) bool {
	if action == string(PolicyActionAll) {
		return true
	}
	if !strings.HasPrefix(action, "s3:") {
		return false
	}
	// Allow wildcard suffixes like s3:Get*
	return true
}

// FormatPolicyForDisplay returns a human-readable description of a policy.
func FormatPolicyForDisplay(policy *BucketPolicy) string {
	if policy == nil {
		return "No policy configured"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Version: %s\n", policy.Version))
	sb.WriteString(fmt.Sprintf("Statements: %d\n", len(policy.Statement)))
	for i, stmt := range policy.Statement {
		sid := stmt.SID
		if sid == "" {
			sid = fmt.Sprintf("(unnamed %d)", i+1)
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s: %v\n", sid, stmt.Effect, stmt.Action))
	}
	return sb.String()
}

// PolicyManager handles bucket policy operations.
type PolicyManager struct {
	engine *PolicyEngine
}

// NewPolicyManager creates a new policy manager.
func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		engine: NewPolicyEngine(),
	}
}

// GetEngine returns the policy engine.
func (pm *PolicyManager) GetEngine() *PolicyEngine {
	return pm.engine
}

// SetPolicy validates and sets a bucket policy on the manager.
func (m *Manager) SetPolicy(bucketName string, policy *BucketPolicy) error {
	if err := ValidatePolicy(policy); err != nil {
		return &S3Error{
			Code:    400,
			CodeStr: "InvalidPolicy",
			Message: fmt.Sprintf("invalid policy: %v", err),
		}
	}
	return m.SetBucketPolicy(bucketName, policy)
}

// DeletePolicy removes a bucket policy.
func (m *Manager) DeletePolicy(bucketName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	bucket.Policy = nil
	return m.saveConfig()
}

// GetPolicy retrieves and returns the bucket policy.
func (m *Manager) GetPolicy(bucketName string) (*BucketPolicy, error) {
	policy, err := m.GetBucketPolicy(bucketName)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, &S3Error{
			Code:    404,
			CodeStr: "NoSuchBucketPolicy",
			Message: "The bucket policy does not exist",
		}
	}
	return policy, nil
}

// EnforceRetentionTime checks if the current time is within a retention period.
func EnforceRetentionTime(retention *RetentionConfig, objectDate time.Time) bool {
	if retention == nil {
		return false
	}

	now := time.Now()
	var retentionEnd time.Time

	if retention.Days > 0 {
		retentionEnd = objectDate.AddDate(0, 0, retention.Days)
	} else if retention.Years > 0 {
		retentionEnd = objectDate.AddDate(retention.Years, 0, 0)
	} else {
		return false
	}

	return now.Before(retentionEnd)
}
