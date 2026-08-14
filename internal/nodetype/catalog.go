// Package nodetype is the typed field catalog for AM journey nodes.
//
// Every attribute we are willing to send or accept is listed here. Generate
// and Read both reject unknown API keys so an AIC upgrade that adds or
// renames a field becomes a compile-or-plan failure, not a silent JSON
// passthrough. That is deliberate: the pain is how the provider stays honest.
package nodetype

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

type Kind int

const (
	KindString Kind = iota
	KindBool
	KindInt
	KindStringList
	KindStringMap
	KindChildren // PageNode.nodes
	KindESVString
)

type Field struct {
	APIName   string
	TFName    string
	Kind      Kind
	Default   any
	Required  bool
	Sensitive bool
	Prefixed  bool // apply provider resource_prefix (InnerTreeEvaluatorNode.tree)
	OmitEmpty bool // skip in HCL when empty / zero / default
}

type Spec struct {
	APIType      string
	TFResource   string // pingoneaic_<this>
	FriendlyName string
	Fields       []Field
}

func (s Spec) FieldByAPI(name string) (Field, bool) {
	for _, f := range s.Fields {
		if f.APIName == name {
			return f, true
		}
	}
	return Field{}, false
}

func (s Spec) FieldByTF(name string) (Field, bool) {
	for _, f := range s.Fields {
		if f.TFName == name {
			return f, true
		}
	}
	return Field{}, false
}

var serverOwned = map[string]struct{}{
	"_id": {}, "_rev": {}, "_type": {}, "_outcomes": {},
}

func All() []Spec {
	return specs
}

func Lookup(apiType string) (Spec, bool) {
	for _, s := range specs {
		if s.APIType == apiType {
			return s, true
		}
	}
	return Spec{}, false
}

func ResourceTypeName(apiType string) string {
	if s, ok := Lookup(apiType); ok {
		return "pingoneaic_" + s.TFResource
	}
	return ""
}

func snakeFromCamel(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func nodeResourceName(apiType string) string {
	name := strings.TrimSuffix(apiType, "Node")
	return snakeFromCamel(name) + "_node"
}

func f(api string, kind Kind, def any) Field {
	return Field{
		APIName:   api,
		TFName:    snakeFromCamel(api),
		Kind:      kind,
		Default:   def,
		OmitEmpty: def != nil,
	}
}

func req(api string, kind Kind, def any) Field {
	fl := f(api, kind, def)
	fl.Required = def == nil
	return fl
}

var specs = []Spec{
	{
		APIType: "ScriptedDecisionNode", TFResource: "scripted_decision_node", FriendlyName: "Scripted Decision",
		Fields: []Field{
			{APIName: "script", TFName: "script_id", Kind: KindString, Required: true},
			{APIName: "outcomes", TFName: "outcomes", Kind: KindStringList, Required: true},
			{APIName: "inputs", TFName: "inputs", Kind: KindStringList, Default: []any{"*"}, OmitEmpty: true},
			{APIName: "outputs", TFName: "outputs", Kind: KindStringList, Default: []any{"*"}, OmitEmpty: true},
		},
	},
	{
		APIType: "MessageNode", TFResource: "message_node", FriendlyName: "Message",
		Fields: []Field{
			f("message", KindStringMap, map[string]any{}),
			f("messageYes", KindStringMap, map[string]any{}),
			f("messageNo", KindStringMap, map[string]any{}),
			{APIName: "stateField", TFName: "state_field", Kind: KindString, OmitEmpty: true},
		},
	},
	{
		APIType: "PageNode", TFResource: "page_node", FriendlyName: "Page",
		Fields: []Field{
			{APIName: "nodes", TFName: "page_nodes", Kind: KindChildren, Required: true},
			{APIName: "pageHeader", TFName: "page_header", Kind: KindStringMap, OmitEmpty: true},
			{APIName: "pageDescription", TFName: "page_description", Kind: KindStringMap, OmitEmpty: true},
			{APIName: "stage", TFName: "stage", Kind: KindString, OmitEmpty: true},
		},
	},
	{
		APIType: "InnerTreeEvaluatorNode", TFResource: "inner_tree_evaluator_node", FriendlyName: "Inner Tree Evaluator",
		Fields: []Field{
			{APIName: "tree", TFName: "tree", Kind: KindString, Required: true, Prefixed: true},
			f("displayErrorOutcome", KindBool, false),
		},
	},
	{
		APIType: "IdentifyExistingUserNode", TFResource: "identify_existing_user_node", FriendlyName: "Identify Existing User",
		Fields: []Field{
			f("identifier", KindString, "userName"),
			f("identityAttribute", KindString, "mail"),
		},
	},
	{
		APIType: "PatchObjectNode", TFResource: "patch_object_node", FriendlyName: "Patch Object",
		Fields: []Field{
			f("identityAttribute", KindString, "userName"),
			f("identityResource", KindString, "managed/user"),
			f("ignoredFields", KindStringList, []any{}),
			f("patchAsObject", KindBool, false),
		},
	},
	{
		APIType: "DataStoreDecisionNode", TFResource: "data_store_decision_node", FriendlyName: "Data Store Decision",
	},
	{
		APIType: "AgentDataStoreDecisionNode", TFResource: "agent_data_store_decision_node", FriendlyName: "Agent Data Store Decision",
	},
	{
		APIType: "AcceptTermsAndConditionsNode", TFResource: "accept_terms_and_conditions_node", FriendlyName: "Accept Terms and Conditions",
	},
	{
		APIType: "PushResultVerifierNode", TFResource: "push_result_verifier_node", FriendlyName: "Push Result Verifier",
	},
	{
		APIType: "OneTimePasswordGeneratorNode", TFResource: "otp_generator_node", FriendlyName: "One Time Password Generator",
		Fields: []Field{f("length", KindInt, float64(8))},
	},
	{
		APIType: "OneTimePasswordCollectorDecisionNode", TFResource: "otp_collector_node", FriendlyName: "One Time Password Collector",
		Fields: []Field{f("passwordExpiryTime", KindInt, float64(5))},
	},
	{
		APIType: "PushRegistrationNode", TFResource: "push_registration_node", FriendlyName: "Push Registration",
		Fields: []Field{
			f("accountName", KindString, "USERNAME"),
			f("bgColor", KindString, "032b75"),
			f("generateRecoveryCodes", KindBool, true),
			f("imgUrl", KindString, ""),
			f("issuer", KindString, "ForgeRock"),
			f("scanQRCodeMessage", KindStringMap, map[string]any{}),
			f("timeout", KindInt, float64(60)),
		},
	},
	{
		APIType: "PushAuthenticationSenderNode", TFResource: "push_sender_node", FriendlyName: "Push Authentication Sender",
		Fields: []Field{
			f("captureFailure", KindBool, false),
			f("contextInfo", KindBool, false),
			f("customPayload", KindStringList, []any{}),
			f("mandatory", KindBool, false),
			f("messageTimeout", KindInt, float64(120000)),
			f("pushType", KindString, "DEFAULT"),
			f("userMessage", KindStringMap, map[string]any{}),
		},
	},
	{
		APIType: "GetAuthenticatorAppNode", TFResource: "get_authenticator_app_node", FriendlyName: "Get Authenticator App",
		Fields: []Field{
			f("appleLink", KindString, "https://apps.apple.com/app/pingid/id891247102"),
			f("googleLink", KindString, "https://play.google.com/store/apps/details?id=prod.com.pingidentity.pingid"),
			f("continueLabel", KindStringMap, map[string]any{}),
			f("message", KindStringMap, map[string]any{}),
		},
	},
	{
		APIType: "RetryLimitDecisionNode", TFResource: "retry_limit_node", FriendlyName: "Retry Limit Decision",
		Fields: []Field{
			f("retryLimit", KindInt, float64(3)),
			f("incrementUserAttributeOnFailure", KindBool, true),
		},
	},
	{
		APIType: "AttributeCollectorNode", TFResource: "attribute_collector_node", FriendlyName: "Attribute Collector",
		Fields: []Field{
			{APIName: "attributesToCollect", TFName: "attributes_to_collect", Kind: KindStringList, Required: true},
			f("identityAttribute", KindString, "userName"),
			f("required", KindBool, false),
			f("validateInputs", KindBool, false),
		},
	},
	{
		APIType: "PersistentCookieDecisionNode", TFResource: "persistent_cookie_decision_node", FriendlyName: "Persistent Cookie Decision",
		Fields: []Field{
			{APIName: "hmacSigningKey", TFName: "hmac_signing_key", Kind: KindESVString, Sensitive: true},
			{APIName: "hmacSigningKeySecretLabelIdentifier", TFName: "hmac_signing_key_secret_label", Kind: KindString, OmitEmpty: true},
			f("idleTimeout", KindInt, float64(5)),
			f("persistentCookieName", KindString, "session-jwt"),
			f("sameSite", KindString, "LAX"),
			f("enforceClientIp", KindBool, false),
			f("useHttpOnlyCookie", KindBool, true),
			f("useSecureCookie", KindBool, true),
		},
	},
	{
		APIType: "SetPersistentCookieNode", TFResource: "set_persistent_cookie_node", FriendlyName: "Set Persistent Cookie",
		Fields: []Field{
			{APIName: "hmacSigningKey", TFName: "hmac_signing_key", Kind: KindESVString, Sensitive: true},
			{APIName: "hmacSigningKeySecretLabelIdentifier", TFName: "hmac_signing_key_secret_label", Kind: KindString, OmitEmpty: true},
			f("idleTimeout", KindInt, float64(5)),
			f("maxLife", KindInt, float64(5)),
			f("persistentCookieName", KindString, "session-jwt"),
			f("sameSite", KindString, "LAX"),
			f("useHttpOnlyCookie", KindBool, true),
			f("useSecureCookie", KindBool, true),
		},
	},
	{
		APIType: "SetSuccessUrlNode", TFResource: "set_success_url_node", FriendlyName: "Set Success URL",
		Fields: []Field{req("successUrl", KindString, nil)},
	},
	{
		APIType: "SetSessionPropertiesNode", TFResource: "set_session_properties_node", FriendlyName: "Set Session Properties",
		Fields: []Field{
			f("properties", KindStringMap, map[string]any{}),
			{APIName: "maxIdleTime", TFName: "max_idle_time", Kind: KindInt, OmitEmpty: true},
			{APIName: "maxSessionTime", TFName: "max_session_time", Kind: KindInt, OmitEmpty: true},
		},
	},
	{
		APIType: "CreateObjectNode", TFResource: "create_object_node", FriendlyName: "Create Object",
		Fields: []Field{f("identityResource", KindString, "managed/user")},
	},
	{
		APIType: "AnonymousUserNode", TFResource: "anonymous_user_node", FriendlyName: "Anonymous User",
		Fields: []Field{f("anonymousUserName", KindString, "anonymous")},
	},
	{
		APIType: "ZeroPageLoginNode", TFResource: "zero_page_login_node", FriendlyName: "Zero Page Login",
		Fields: []Field{
			f("allowWithoutReferer", KindBool, true),
			f("passwordHeader", KindString, "X-OpenAM-Password"),
			f("usernameHeader", KindString, "X-OpenAM-Username"),
			f("referrerWhiteList", KindStringList, []any{}),
		},
	},
	{
		APIType: "PassthroughAuthenticationNode", TFResource: "passthrough_auth_node", FriendlyName: "Passthrough Authentication",
		Fields: []Field{
			req("systemEndpoint", KindString, nil),
			f("objectType", KindString, "account"),
			f("passwordAttribute", KindString, "password"),
			f("identityAttribute", KindString, "userName"),
		},
	},
	{
		APIType: "SessionDataNode", TFResource: "session_data_node", FriendlyName: "Session Data",
		Fields: []Field{
			req("sessionDataKey", KindString, nil),
			req("sharedStateKey", KindString, nil),
		},
	},
	{
		APIType: "SetStateNode", TFResource: "set_state_node", FriendlyName: "Set State",
		Fields: []Field{f("attributes", KindStringMap, map[string]any{})},
	},
	{
		APIType: "ModifyAuthLevelNode", TFResource: "modify_auth_level_node", FriendlyName: "Modify Auth Level",
		Fields: []Field{f("authLevelIncrement", KindInt, float64(0))},
	},
	{
		APIType: "ChoiceCollectorNode", TFResource: "choice_collector_node", FriendlyName: "Choice Collector",
		Fields: []Field{
			{APIName: "choices", TFName: "choices", Kind: KindStringList, Required: true},
			req("defaultChoice", KindString, nil),
			req("prompt", KindString, nil),
		},
	},
	{
		APIType: "PollingWaitNode", TFResource: "polling_wait_node", FriendlyName: "Polling Wait",
		Fields: []Field{
			f("waitingMessage", KindStringMap, map[string]any{}),
			f("exitMessage", KindStringMap, map[string]any{}),
			f("exitable", KindBool, false),
			f("secondsToWait", KindInt, float64(8)),
			f("spamDetectionEnabled", KindBool, false),
			f("spamDetectionTolerance", KindInt, float64(3)),
		},
	},
	{
		APIType: "UsernameCollectorNode", TFResource: "username_collector_node", FriendlyName: "Username Collector",
		Fields: []Field{
			f("autocompleteValues", KindStringList, []any{}),
		},
	},
	{
		APIType: "PasswordCollectorNode", TFResource: "password_collector_node", FriendlyName: "Password Collector",
	},
	{
		APIType: "ValidatedUsernameNode", TFResource: "validated_username_node", FriendlyName: "Validated Username",
		Fields: []Field{
			f("usernameAttribute", KindString, "userName"),
			f("validateInput", KindBool, false),
			f("autocompleteValues", KindStringList, []any{}),
		},
	},
	{
		APIType: "ValidatedPasswordNode", TFResource: "validated_password_node", FriendlyName: "Validated Password",
		Fields: []Field{
			f("passwordAttribute", KindString, "password"),
			f("validateInput", KindBool, false),
		},
	},
}

func init() {
	// Keep TF resource names stable even if the helper would drift.
	_ = nodeResourceName
}

// DecodeAPI turns a raw node GET body into terraform-facing values.
// Unknown keys (other than server-owned metadata) are an error.
func DecodeAPI(spec Spec, raw map[string]any, resourcePrefix string) (map[string]any, error) {
	var unknown []string
	out := make(map[string]any)
	for k, v := range raw {
		if _, skip := serverOwned[k]; skip {
			continue
		}
		field, ok := spec.FieldByAPI(k)
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		decoded, err := decodeValue(field, v, resourcePrefix)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", spec.APIType, k, err)
		}
		out[field.TFName] = decoded
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%s returned unmodelled fields %v — add them to internal/nodetype/catalog.go", spec.APIType, unknown)
	}
	return out, nil
}

// EncodeAPI builds a PUT body from terraform-facing values, filling defaults
// for omitted optional fields so AM's "required" list is satisfied.
func EncodeAPI(spec Spec, tf map[string]any, resourcePrefix string) (map[string]any, error) {
	out := make(map[string]any)
	for _, field := range spec.Fields {
		v, set := tf[field.TFName]
		if !set || isNil(v) {
			if field.Default != nil {
				v = cloneDefault(field.Default)
			} else if field.Required {
				return nil, fmt.Errorf("%s: %s is required", spec.APIType, field.TFName)
			} else {
				continue
			}
		}
		encoded, err := encodeValue(field, v, resourcePrefix)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", spec.APIType, field.TFName, err)
		}
		out[field.APIName] = encoded
	}
	// Reject TF keys we do not know. Prevents silently dropping typos.
	for k := range tf {
		if k == "id" || k == "realm" || k == "type" {
			continue
		}
		if _, ok := spec.FieldByTF(k); !ok {
			return nil, fmt.Errorf("%s: unknown terraform attribute %q", spec.APIType, k)
		}
	}
	return out, nil
}

func isNil(v any) bool {
	return v == nil
}

func cloneDefault(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func decodeValue(field Field, v any, resourcePrefix string) (any, error) {
	switch field.Kind {
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		if field.Prefixed {
			return prefix.Strip(resourcePrefix, s), nil
		}
		return s, nil
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case KindInt:
		switch n := v.(type) {
		case float64:
			return int64(n), nil
		case json.Number:
			i, err := n.Int64()
			return i, err
		case int:
			return int64(n), nil
		case int64:
			return n, nil
		default:
			return nil, fmt.Errorf("expected number, got %T", v)
		}
	case KindStringList:
		arr, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array, got %T", v)
		}
		out := make([]string, 0, len(arr))
		for i, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("[%d]: expected string, got %T", i, item)
			}
			out = append(out, s)
		}
		return out, nil
	case KindStringMap:
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object, got %T", v)
		}
		out := make(map[string]string, len(obj))
		for k, item := range obj {
			switch t := item.(type) {
			case string:
				out[k] = t
			case nil:
				out[k] = ""
			default:
				// AIC sometimes stores numbers in "properties". Refuse so we
				// learn about it rather than stringify silently.
				return nil, fmt.Errorf("[%s]: expected string, got %T", k, item)
			}
		}
		return out, nil
	case KindChildren:
		arr, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("expected child array, got %T", v)
		}
		var out []PageChild
		for i, item := range arr {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("[%d]: expected object, got %T", i, item)
			}
			var extra []string
			child := PageChild{}
			for k, val := range obj {
				switch k {
				case "_id":
					child.ID, _ = val.(string)
				case "displayName":
					child.DisplayName, _ = val.(string)
				case "nodeType":
					child.NodeType, _ = val.(string)
				case "nodeVersion":
					child.NodeVersion, _ = val.(string)
				default:
					extra = append(extra, k)
				}
			}
			if len(extra) > 0 {
				sort.Strings(extra)
				return nil, fmt.Errorf("page child has unmodelled fields %v", extra)
			}
			if child.ID == "" || child.NodeType == "" {
				return nil, fmt.Errorf("[%d]: child missing _id or nodeType", i)
			}
			out = append(out, child)
		}
		return out, nil
	case KindESVString:
		s, err := unwrapESVString(v)
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unhandled field kind %d", field.Kind)
	}
}

func encodeValue(field Field, v any, resourcePrefix string) (any, error) {
	switch field.Kind {
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		if field.Prefixed {
			return prefix.Apply(resourcePrefix, s), nil
		}
		return s, nil
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case KindInt:
		switch n := v.(type) {
		case int64:
			return n, nil
		case int:
			return int64(n), nil
		case float64:
			return int64(n), nil
		default:
			return nil, fmt.Errorf("expected int, got %T", v)
		}
	case KindStringList:
		switch t := v.(type) {
		case []string:
			out := make([]any, len(t))
			for i, s := range t {
				out[i] = s
			}
			return out, nil
		case []any:
			return t, nil
		default:
			return nil, fmt.Errorf("expected string list, got %T", v)
		}
	case KindStringMap:
		switch t := v.(type) {
		case map[string]string:
			out := make(map[string]any, len(t))
			for k, val := range t {
				out[k] = val
			}
			return out, nil
		case map[string]any:
			return t, nil
		default:
			return nil, fmt.Errorf("expected string map, got %T", v)
		}
	case KindChildren:
		var children []PageChild
		switch t := v.(type) {
		case []PageChild:
			children = t
		default:
			return nil, fmt.Errorf("expected page children, got %T", v)
		}
		out := make([]any, 0, len(children))
		for _, ch := range children {
			ver := ch.NodeVersion
			if ver == "" {
				ver = "1.0"
			}
			out = append(out, map[string]any{
				"_id":         ch.ID,
				"displayName": ch.DisplayName,
				"nodeType":    ch.NodeType,
				"nodeVersion": ver,
			})
		}
		return out, nil
	case KindESVString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		return wrapESVString(s), nil
	default:
		return nil, fmt.Errorf("unhandled field kind %d", field.Kind)
	}
}

type PageChild struct {
	ID          string
	DisplayName string
	NodeType    string
	NodeVersion string
}

// unwrapESVString accepts either a plain string or AM's
// `{"$string": "&{esv.foo}"}` placeholder wrapper. Anything else is an error
// so a new secret-encoding shape forces a provider change.
func unwrapESVString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case map[string]any:
		if s, ok := t["$string"].(string); ok && len(t) == 1 {
			return s, nil
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("unrecognised ESV/secret wrapper keys %v", keys)
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("expected string or {$string: ...}, got %T", v)
	}
}

func wrapESVString(s string) any {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "&{") && strings.HasSuffix(s, "}") {
		return map[string]any{"$string": s}
	}
	return s
}

// EqualDefault reports whether v is the catalog default (so generate can omit it).
func EqualDefault(field Field, v any) bool {
	if field.Default == nil {
		if field.Kind == KindStringMap {
			m, ok := v.(map[string]string)
			return ok && len(m) == 0
		}
		if field.Kind == KindStringList {
			s, ok := v.([]string)
			return ok && len(s) == 0
		}
		if field.Kind == KindString || field.Kind == KindESVString {
			s, ok := v.(string)
			return ok && s == ""
		}
		return false
	}
	got, _ := json.Marshal(normalizeForCompare(v))
	want, _ := json.Marshal(normalizeForCompare(field.Default))
	return string(got) == string(want)
}

func normalizeForCompare(v any) any {
	switch t := v.(type) {
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = val
		}
		return out
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return v
	}
}
