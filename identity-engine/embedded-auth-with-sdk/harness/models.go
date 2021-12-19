package harness

import (
	"github.com/okta/okta-sdk-golang/v2/okta"
	"time"
)

type Policy struct {
	Embedded    interface{}                `json:"_embedded,omitempty"`
	Links       interface{}                `json:"_links,omitempty"`
	Conditions  *okta.PolicyRuleConditions `json:"conditions,omitempty"`
	Created     *time.Time                 `json:"created,omitempty"`
	Description string                     `json:"description,omitempty"`
	Id          string                     `json:"id,omitempty"`
	LastUpdated *time.Time                 `json:"lastUpdated,omitempty"`
	Name        string                     `json:"name,omitempty"`
	Priority    int64                      `json:"priority,omitempty"`
	Status      string                     `json:"status,omitempty"`
	System      *bool                      `json:"system,omitempty"`
	Type        string                     `json:"type,omitempty"`
	Settings    *PolicySettings            `json:"settings,omitempty"`
}

type PolicySettings struct {
	Factors    *PolicyFactorsSettings                 `json:"factors,omitempty"`
	Delegation *okta.PasswordPolicyDelegationSettings `json:"delegation,omitempty"`
	Password   *PasswordPolicyPasswordSettings        `json:"password,omitempty"`
	Recovery   *PasswordPolicyRecoverySettings        `json:"recovery,omitempty"`
}

type PasswordPolicyPasswordSettings struct {
	Age        *PasswordPolicyPasswordSettingsAge        `json:"age,omitempty"`
	Complexity *PasswordPolicyPasswordSettingsComplexity `json:"complexity,omitempty"`
	Lockout    *PasswordPolicyPasswordSettingsLockout    `json:"lockout,omitempty"`
}

type PasswordPolicyPasswordSettingsAge struct {
	ExpireWarnDays int64 `json:"expireWarnDays"`
	HistoryCount   int64 `json:"historyCount"`
	MaxAgeDays     int64 `json:"maxAgeDays"`
	MinAgeMinutes  int64 `json:"minAgeMinutes"`
}

type PasswordPolicyPasswordSettingsComplexity struct {
	Dictionary        *okta.PasswordDictionary `json:"dictionary,omitempty"`
	ExcludeAttributes []string                 `json:"excludeAttributes,omitempty"`
	ExcludeUsername   *bool                    `json:"excludeUsername,omitempty"`
	MinLength         int64                    `json:"minLength"`
	MinLowerCase      int64                    `json:"minLowerCase"`
	MinNumber         int64                    `json:"minNumber"`
	MinSymbol         int64                    `json:"minSymbol"`
	MinUpperCase      int64                    `json:"minUpperCase"`
}

type PasswordPolicyRecoverySettings struct {
	Factors *PasswordPolicyRecoveryFactors `json:"factors,omitempty"`
}

type PasswordPolicyRecoveryFactors struct {
	OktaCall         *okta.PasswordPolicyRecoveryFactorSettings `json:"okta_call,omitempty"`
	OktaSms          *okta.PasswordPolicyRecoveryFactorSettings `json:"okta_sms,omitempty"`
	OktaEmail        *PasswordPolicyRecoveryEmail               `json:"okta_email,omitempty"`
	RecoveryQuestion *PasswordPolicyRecoveryQuestion            `json:"recovery_question,omitempty"`
}

type PasswordPolicyRecoveryEmail struct {
	Properties *PasswordPolicyRecoveryEmailProperties `json:"properties,omitempty"`
	Status     string                                 `json:"status,omitempty"`
}

type PasswordPolicyRecoveryEmailProperties struct {
	RecoveryToken *PasswordPolicyRecoveryEmailRecoveryToken `json:"recoveryToken,omitempty"`
}

type PasswordPolicyRecoveryEmailRecoveryToken struct {
	TokenLifetimeMinutes int64 `json:"tokenLifetimeMinutes"`
}

type PasswordPolicyRecoveryQuestion struct {
	Properties *PasswordPolicyRecoveryQuestionProperties `json:"properties,omitempty"`
	Status     string                                    `json:"status,omitempty"`
}

type PasswordPolicyRecoveryQuestionProperties struct {
	Complexity *PasswordPolicyRecoveryQuestionComplexity `json:"complexity,omitempty"`
}

type PasswordPolicyPasswordSettingsLockout struct {
	AutoUnlockMinutes               int64    `json:"autoUnlockMinutes"`
	MaxAttempts                     int64    `json:"maxAttempts"`
	ShowLockoutFailures             *bool    `json:"showLockoutFailures,omitempty"`
	UserLockoutNotificationChannels []string `json:"userLockoutNotificationChannels,omitempty"`
}

type PasswordPolicyRecoveryQuestionComplexity struct {
	MinLength int64 `json:"minLength"`
}

type PolicyFactorsSettings struct {
	Duo          *PolicyFactor `json:"duo,omitempty"`
	FidoU2f      *PolicyFactor `json:"fido_u2f,omitempty"`
	FidoWebauthn *PolicyFactor `json:"fido_webauthn,omitempty"`
	GoogleOtp    *PolicyFactor `json:"google_otp,omitempty"`
	OktaCall     *PolicyFactor `json:"okta_call,omitempty"`
	OktaOtp      *PolicyFactor `json:"okta_otp,omitempty"`
	OktaPassword *PolicyFactor `json:"okta_password,omitempty"`
	OktaPush     *PolicyFactor `json:"okta_push,omitempty"`
	OktaQuestion *PolicyFactor `json:"okta_question,omitempty"`
	OktaSms      *PolicyFactor `json:"okta_sms,omitempty"`
	OktaEmail    *PolicyFactor `json:"okta_email,omitempty"`
	RsaToken     *PolicyFactor `json:"rsa_token,omitempty"`
	SymantecVip  *PolicyFactor `json:"symantec_vip,omitempty"`
	YubikeyToken *PolicyFactor `json:"yubikey_token,omitempty"`
	Hotp         *PolicyFactor `json:"hotp,omitempty"`
}

type PolicyFactor struct {
	Consent *Consent `json:"consent,omitempty"`
	Enroll  *Enroll  `json:"enroll,omitempty"`
}

type Consent struct {
	Terms *Terms `json:"terms,omitempty"`
	Type  string `json:"type,omitempty"`
}

type Terms struct {
	Format string `json:"format,omitempty"`
	Value  string `json:"value,omitempty"`
}

type Enroll struct {
	Self string `json:"self,omitempty"`
}
