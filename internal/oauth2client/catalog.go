package oauth2client

// Package oauth2client is the typed field catalog for AM OAuth2 clients.
//
// Every attribute we are willing to send or accept is listed here.
// Generate and Read both reject unknown API keys so an AIC upgrade
// that adds or renames a field becomes a plan failure, not a silent
// JSON passthrough.
//
// Field names and defaults were taken from the live tenant template and
// schema (POST …/OAuth2Client?_action=template|schema) on 2026-08-15,
// then checked against a GET of every OAuth2 client in realm alpha.
//
// Two template defaults are lists of a single empty string (`[""]`):
// allowedResourceServerAudienceValues and customProperties. AM stores
// those as `[]` on read-back, so the catalog default is the stored form.

type Kind int

const (
	KindString Kind = iota
	KindBool
	KindInt
	KindStringList
)

type Field struct {
	APIName   string
	TFName    string
	Kind      Kind
	Default   any
	Sensitive bool
	Prefixed  bool // apply resource_prefix (treeName)
	Wrapped   bool // GET wraps the value in {inherited,value}
	OmitEmpty bool
}

type Group struct {
	APIName string
	TFName  string
	Doc     string
	Fields  []Field
}

func (g Group) FieldByAPI(name string) (Field, bool) {
	for _, f := range g.Fields {
		if f.APIName == name {
			return f, true
		}
	}
	return Field{}, false
}

func (g Group) FieldByTF(name string) (Field, bool) {
	for _, f := range g.Fields {
		if f.TFName == name {
			return f, true
		}
	}
	return Field{}, false
}

func AllGroups() []Group { return groups }

func LookupGroup(api string) (Group, bool) {
	for _, g := range groups {
		if g.APIName == api {
			return g, true
		}
	}
	return Group{}, false
}

func FieldCount() int {
	n := 0
	for _, g := range groups {
		n += len(g.Fields)
	}
	return n
}

var groups = []Group{
	{
		APIName: "coreOAuth2ClientConfig", TFName: "core", Doc: "Core OAuth2 settings (client type, scopes, lifetimes).",
		Fields: []Field{
			{APIName: "accessTokenLifetime", TFName: "access_token_lifetime", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "agentgroup", TFName: "agentgroup", Kind: KindString, Default: nil, OmitEmpty: true},
			{APIName: "authorizationCodeLifetime", TFName: "authorization_code_lifetime", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "authorizationDetailTypes", TFName: "authorization_detail_types", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "clientName", TFName: "client_name", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "clientType", TFName: "client_type", Kind: KindString, Default: "Confidential", Wrapped: true},
			{APIName: "defaultScopes", TFName: "default_scopes", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "loopbackInterfaceRedirection", TFName: "loopback_interface_redirection", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "redirectionUris", TFName: "redirection_uris", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "refreshTokenLifetime", TFName: "refresh_token_lifetime", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "scopes", TFName: "scopes", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "secretLabelIdentifier", TFName: "secret_label_identifier", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "status", TFName: "status", Kind: KindString, Default: "Active", Wrapped: true},
			{APIName: "userpassword", TFName: "userpassword", Kind: KindString, Default: nil, Sensitive: true, OmitEmpty: true},
		},
	},
	{
		APIName: "advancedOAuth2ClientConfig", TFName: "advanced", Doc: "Advanced OAuth2 settings (grants, response types, origins).",
		Fields: []Field{
			{APIName: "acceptedJwtIssuers", TFName: "accepted_jwt_issuers", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "allowedResourceServerAudienceValues", TFName: "allowed_resource_server_audience_values", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "clientUri", TFName: "client_uri", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "contacts", TFName: "contacts", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "customProperties", TFName: "custom_properties", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "descriptions", TFName: "descriptions", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "grantTypes", TFName: "grant_types", Kind: KindStringList, Default: []any{"authorization_code"}, Wrapped: true},
			{APIName: "introspectionPolicySets", TFName: "introspection_policy_sets", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "isConsentImplied", TFName: "is_consent_implied", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "javascriptOrigins", TFName: "javascript_origins", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "logoUri", TFName: "logo_uri", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "mixUpMitigation", TFName: "mix_up_mitigation", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "name", TFName: "localized_names", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "policyUri", TFName: "policy_uri", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "refreshTokenGracePeriod", TFName: "refresh_token_grace_period", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "requestUris", TFName: "request_uris", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "require_pushed_authorization_requests", TFName: "require_pushed_authorization_requests", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "responseTypes", TFName: "response_types", Kind: KindStringList, Default: []any{"code", "token", "id_token", "code token", "token id_token", "code id_token", "code token id_token", "device_code", "device_code id_token"}, Wrapped: true},
			{APIName: "sectorIdentifierUri", TFName: "sector_identifier_uri", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "softwareIdentity", TFName: "software_identity", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "softwareVersion", TFName: "software_version", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "subjectType", TFName: "subject_type", Kind: KindString, Default: "public", Wrapped: true},
			{APIName: "tlsCaSecretLabelIdentifier", TFName: "tls_ca_secret_label_identifier", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "tokenEndpointAuthMethod", TFName: "token_endpoint_auth_method", Kind: KindString, Default: "client_secret_basic", Wrapped: true},
			{APIName: "tokenExchangeAuthLevel", TFName: "token_exchange_auth_level", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "tosURI", TFName: "tos_uri", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "treeName", TFName: "tree_name", Kind: KindString, Default: "[Empty]", Prefixed: true, Wrapped: true},
			{APIName: "updateAccessToken", TFName: "update_access_token", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
		},
	},
	{
		APIName: "signEncOAuth2ClientConfig", TFName: "sign_enc", Doc: "Signing and encryption settings.",
		Fields: []Field{
			{APIName: "authorizationResponseEncryptionAlgorithm", TFName: "authorization_response_encryption_algorithm", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "authorizationResponseEncryptionMethod", TFName: "authorization_response_encryption_method", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "authorizationResponseSigningAlgorithm", TFName: "authorization_response_signing_algorithm", Kind: KindString, Default: "RS256", Wrapped: true},
			{APIName: "clientJwtPublicKey", TFName: "client_jwt_public_key", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "idTokenEncryptionAlgorithm", TFName: "id_token_encryption_algorithm", Kind: KindString, Default: "RSA-OAEP-256", Wrapped: true},
			{APIName: "idTokenEncryptionEnabled", TFName: "id_token_encryption_enabled", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "idTokenEncryptionMethod", TFName: "id_token_encryption_method", Kind: KindString, Default: "A128CBC-HS256", Wrapped: true},
			{APIName: "idTokenPublicEncryptionKey", TFName: "id_token_public_encryption_key", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "idTokenSignedResponseAlg", TFName: "id_token_signed_response_alg", Kind: KindString, Default: "RS256", Wrapped: true},
			{APIName: "jwkSet", TFName: "jwk_set", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "jwkStoreCacheMissCacheTime", TFName: "jwk_store_cache_miss_cache_time", Kind: KindInt, Default: int64(60000), Wrapped: true},
			{APIName: "jwksCacheTimeout", TFName: "jwks_cache_timeout", Kind: KindInt, Default: int64(3600000), Wrapped: true},
			{APIName: "jwksUri", TFName: "jwks_uri", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "mTLSCertificateBoundAccessTokens", TFName: "mtls_certificate_bound_access_tokens", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "mTLSSubjectDN", TFName: "mtls_subject_dn", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "mTLSTrustedCert", TFName: "mtls_trusted_cert", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "publicKeyLocation", TFName: "public_key_location", Kind: KindString, Default: "jwks_uri", Wrapped: true},
			{APIName: "requestParameterEncryptedAlg", TFName: "request_parameter_encrypted_alg", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "requestParameterEncryptedEncryptionAlgorithm", TFName: "request_parameter_encrypted_encryption_algorithm", Kind: KindString, Default: "A128CBC-HS256", Wrapped: true},
			{APIName: "requestParameterSignedAlg", TFName: "request_parameter_signed_alg", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "tokenEndpointAuthSigningAlgorithm", TFName: "token_endpoint_auth_signing_algorithm", Kind: KindString, Default: "RS256", Wrapped: true},
			{APIName: "tokenIntrospectionEncryptedResponseAlg", TFName: "token_introspection_encrypted_response_alg", Kind: KindString, Default: "RSA-OAEP-256", Wrapped: true},
			{APIName: "tokenIntrospectionEncryptedResponseEncryptionAlgorithm", TFName: "token_introspection_encrypted_response_encryption_algorithm", Kind: KindString, Default: "A128CBC-HS256", Wrapped: true},
			{APIName: "tokenIntrospectionResponseFormat", TFName: "token_introspection_response_format", Kind: KindString, Default: "JSON", Wrapped: true},
			{APIName: "tokenIntrospectionSignedResponseAlg", TFName: "token_introspection_signed_response_alg", Kind: KindString, Default: "RS256", Wrapped: true},
			{APIName: "userinfoEncryptedResponseAlg", TFName: "userinfo_encrypted_response_alg", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "userinfoEncryptedResponseEncryptionAlgorithm", TFName: "userinfo_encrypted_response_encryption_algorithm", Kind: KindString, Default: "A128CBC-HS256", Wrapped: true},
			{APIName: "userinfoResponseFormat", TFName: "userinfo_response_format", Kind: KindString, Default: "JSON", Wrapped: true},
			{APIName: "userinfoSignedResponseAlg", TFName: "userinfo_signed_response_alg", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
		},
	},
	{
		APIName: "coreOpenIDClientConfig", TFName: "oidc", Doc: "OpenID Connect settings.",
		Fields: []Field{
			{APIName: "backchannel_logout_session_required", TFName: "backchannel_logout_session_required", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "backchannel_logout_uri", TFName: "backchannel_logout_uri", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "claims", TFName: "claims", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "clientSessionUri", TFName: "client_session_uri", Kind: KindString, Default: nil, Wrapped: true, OmitEmpty: true},
			{APIName: "defaultAcrValues", TFName: "default_acr_values", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
			{APIName: "defaultMaxAge", TFName: "default_max_age", Kind: KindInt, Default: int64(600), Wrapped: true},
			{APIName: "defaultMaxAgeEnabled", TFName: "default_max_age_enabled", Kind: KindBool, Default: false, Wrapped: true},
			{APIName: "jwtTokenLifetime", TFName: "jwt_token_lifetime", Kind: KindInt, Default: int64(0), Wrapped: true},
			{APIName: "postLogoutRedirectUri", TFName: "post_logout_redirect_uri", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
		},
	},
	{
		APIName: "coreUmaClientConfig", TFName: "uma", Doc: "UMA settings.",
		Fields: []Field{
			{APIName: "claimsRedirectionUris", TFName: "claims_redirection_uris", Kind: KindStringList, Default: []any{}, Wrapped: true, OmitEmpty: true},
		},
	},
	{
		APIName: "overrideOAuth2ClientConfig", TFName: "override", Doc: "Per-client plugin and script overrides. Ignored unless provider_overrides_enabled is true.",
		Fields: []Field{
			{APIName: "acceptAudienceParametersInTokenExchangeRequests", TFName: "accept_audience_parameters_in_token_exchange_requests", Kind: KindBool, Default: false},
			{APIName: "accessTokenMayActScript", TFName: "access_token_may_act_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "accessTokenModificationPluginType", TFName: "access_token_modification_plugin_type", Kind: KindString, Default: "PROVIDER"},
			{APIName: "accessTokenModificationScript", TFName: "access_token_modification_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "accessTokenModifierClass", TFName: "access_token_modifier_class", Kind: KindString, Default: nil, OmitEmpty: true},
			{APIName: "authorizeEndpointDataProviderClass", TFName: "authorize_endpoint_data_provider_class", Kind: KindString, Default: "org.forgerock.oauth2.core.plugins.registry.DefaultEndpointDataProvider"},
			{APIName: "authorizeEndpointDataProviderPluginType", TFName: "authorize_endpoint_data_provider_plugin_type", Kind: KindString, Default: "PROVIDER"},
			{APIName: "authorizeEndpointDataProviderScript", TFName: "authorize_endpoint_data_provider_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "clientsCanSkipConsent", TFName: "clients_can_skip_consent", Kind: KindBool, Default: false},
			{APIName: "customLoginUrlTemplate", TFName: "custom_login_url_template", Kind: KindString, Default: nil, OmitEmpty: true},
			{APIName: "enableApplicationContext", TFName: "enable_application_context", Kind: KindBool, Default: false},
			{APIName: "enableRemoteConsent", TFName: "enable_remote_consent", Kind: KindBool, Default: false},
			{APIName: "evaluateScopeClass", TFName: "evaluate_scope_class", Kind: KindString, Default: "org.forgerock.oauth2.core.plugins.registry.DefaultScopeEvaluator"},
			{APIName: "evaluateScopePluginType", TFName: "evaluate_scope_plugin_type", Kind: KindString, Default: "PROVIDER"},
			{APIName: "evaluateScopeScript", TFName: "evaluate_scope_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "issueRefreshToken", TFName: "issue_refresh_token", Kind: KindBool, Default: true},
			{APIName: "issueRefreshTokenOnRefreshedToken", TFName: "issue_refresh_token_on_refreshed_token", Kind: KindBool, Default: true},
			{APIName: "oidcClaimsClass", TFName: "oidc_claims_class", Kind: KindString, Default: nil, OmitEmpty: true},
			{APIName: "oidcClaimsPluginType", TFName: "oidc_claims_plugin_type", Kind: KindString, Default: "PROVIDER"},
			{APIName: "oidcClaimsScript", TFName: "oidc_claims_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "oidcMayActScript", TFName: "oidc_may_act_script", Kind: KindString, Default: "[Empty]"},
			{APIName: "overrideableOIDCClaims", TFName: "overrideable_oidc_claims", Kind: KindStringList, Default: []any{}, OmitEmpty: true},
			{APIName: "providerOverridesEnabled", TFName: "provider_overrides_enabled", Kind: KindBool, Default: false},
			{APIName: "remoteConsentServiceId", TFName: "remote_consent_service_id", Kind: KindString, Default: nil, OmitEmpty: true},
			{APIName: "scopesPolicySet", TFName: "scopes_policy_set", Kind: KindString, Default: "oauth2Scopes"},
			{APIName: "statelessTokensEnabled", TFName: "stateless_tokens_enabled", Kind: KindBool, Default: false},
			{APIName: "tlsClientCertificateChainValidationEnabled", TFName: "tls_client_certificate_chain_validation_enabled", Kind: KindBool, Default: false},
			{APIName: "tokenEncryptionEnabled", TFName: "token_encryption_enabled", Kind: KindBool, Default: false},
			{APIName: "useForceAuthnForMaxAge", TFName: "use_force_authn_for_max_age", Kind: KindBool, Default: false},
			{APIName: "usePolicyEngineForScope", TFName: "use_policy_engine_for_scope", Kind: KindBool, Default: false},
			{APIName: "useTokenIntrospectionClaimForJWT", TFName: "use_token_introspection_claim_for_jwt", Kind: KindBool, Default: false},
			{APIName: "validateScopeClass", TFName: "validate_scope_class", Kind: KindString, Default: "org.forgerock.oauth2.core.plugins.registry.DefaultScopeValidator"},
			{APIName: "validateScopePluginType", TFName: "validate_scope_plugin_type", Kind: KindString, Default: "PROVIDER"},
			{APIName: "validateScopeScript", TFName: "validate_scope_script", Kind: KindString, Default: "[Empty]"},
		},
	},
}
