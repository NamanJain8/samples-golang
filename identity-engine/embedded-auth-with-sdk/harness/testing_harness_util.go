/**
 * Copyright 2021 - Present Okta, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tebeka/selenium"
)

const (
	ERROR_DIV = `div[class="mx-auto py-4 px-2 my-2 w-full border-2 border-red-400 bg-red-100"]`
)

func a18nApiURL() string {
	url := os.Getenv("A18N_API_URL")
	if url == "" {
		url = "https://api.a18n.help"
	}
	return url
}

func defaultTimeout() time.Duration {
	return time.Duration(time.Second * 10)
}

func defaultInterval() time.Duration {
	return time.Duration(time.Second * 3)
}

func claims() map[string]string {
	claimsJSON := os.Getenv("OKTA_IDX_CLAIMS")
	claims := map[string]string{}
	err := json.Unmarshal([]byte(claimsJSON), &claims)
	if err != nil {
		fmt.Printf("unable to unmarshal env var OKTA_IDX_CLAIMS %q\n", claimsJSON)
		return map[string]string{}
	}
	return claims
}

func claimItem(key string) string {
	value, _ := claims()[key]
	return value
}

func randomString() string {
	digits := "0123456789"
	specials := "~=+%^*/()[]{}/!@#$?|"
	all := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		digits + specials
	length := 12
	buf := make([]byte, length)
	buf[0] = digits[rand.Intn(len(digits))]
	buf[1] = specials[rand.Intn(len(specials))]
	for i := 2; i < length; i++ {
		buf[i] = all[rand.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})
	return string(buf)
}

func (th *TestHarness) navigateToTheRootView() error {
	rootURL := fmt.Sprintf("http://%s/", th.server.Address())
	err := th.wd.Get(rootURL)
	if err != nil {
		return err
	}

	return th.waitForPageRender()
}

func (th *TestHarness) navigateToBasicLogin() error {
	loginURL := fmt.Sprintf("http://%s/login", th.server.Address())
	err := th.wd.Get(loginURL)
	if err != nil {
		return err
	}
	return th.waitForPageRender()
}

func (th *TestHarness) navigateToSelfServiceRegistration() error {
	rootURL := fmt.Sprintf("http://%s/register", th.server.Address())
	err := th.wd.Get(rootURL)
	if err != nil {
		return err
	}

	return th.waitForPageRender()
}

func (th *TestHarness) isRootView() error {
	return th.isView(fmt.Sprintf("http://%s/", th.server.Address()))
}

func (th *TestHarness) isPasswordResetView() error {
	return th.isView(fmt.Sprintf("http://%s/passwordRecovery", th.server.Address()))
}

func (th *TestHarness) isView(url string) error {
	currentURL, err := th.wd.CurrentURL()
	if err != nil {
		return err
	}

	if url != currentURL {
		return fmt.Errorf("isView expects %q url, finds %q url", url, currentURL)
	}

	return nil
}

func (th *TestHarness) waitForPageRender() error {
	return th.seesElement(`html body h1`)
}

func (th *TestHarness) checkEntryPoints() error {
	baseURL := fmt.Sprintf("http://%s", th.server.Address())
	links := []struct {
		text string
		href string
	}{
		{
			text: "Sign In",
			href: fmt.Sprintf("%s/login", baseURL),
		},
		{
			text: "Sign Up",
			href: fmt.Sprintf("%s/register", baseURL),
		},
		{
			text: "Password Recovery",
			href: fmt.Sprintf("%s/passwordRecovery", baseURL),
		},
		{
			text: "Logout",
			href: fmt.Sprintf("%s/logout", baseURL),
		},
	}

	for _, link := range links {
		elem, err := th.wd.FindElement(selenium.ByLinkText, link.text)
		if err != nil {
			return err
		}
		href, err := elem.GetAttribute("href")
		if !strings.HasSuffix(link.href, href) {
			return fmt.Errorf("expected to find link %q with href %q but found it with %q", link.text, link.href, href)
		}
	}

	return nil
}

func (th *TestHarness) waitForLoginForm() error {
	return th.seesElement(`form[action="/login"]`)
}

func (th *TestHarness) waitForRegistrationForm() error {
	return th.seesElement(`form[action="/register"]`)
}

func (th *TestHarness) waitForEnrollPasswordForm() error {
	return th.seesElement(`form[action="/enrollPassword"]`)
}

func (th *TestHarness) waitForEnrollFactorForm() error {
	return th.seesElement(`form[action="/enrollFactor"]`)
}

func (th *TestHarness) waitForEnrollEmailForm() error {
	return th.seesElement(`form[action="/enrollEmail"]`)
}

func (th *TestHarness) waitForEnrollPhoneForm() error {
	return th.seesElement(`form[action="/enrollPhone/code"]`)
}

func (th *TestHarness) waitForEnrollPhoneMethodForm() error {
	return th.seesElement(`form[action="/enrollPhone/method"]`)
}

func (th *TestHarness) seesClaimsTableItemAndValueFromCurrentProfile(key string) error {
	keyID := fmt.Sprintf("%s-value", key)
	var value string
	switch {
	case key == "name":
		value = th.currentProfile.DisplayName
	case key == "email":
		value = th.currentProfile.EmailAddress
	}

	return th.seesElementIDWithValue(keyID, value)
}

func (th *TestHarness) loginToApplication() error {
	err := th.clickLink("Sign In")
	if err != nil {
		return err
	}

	if err = th.waitForPageRender(); err != nil {
		return err
	}

	if err = th.waitForLoginForm(); err != nil {
		return err
	}

	if err = th.fillsInUsername(); err != nil {
		return err
	}

	if err = th.fillsInPassword(); err != nil {
		return err
	}

	if err = th.submitsLoginForm(); err != nil {
		return err
	}

	if err = th.waitForPageRender(); err != nil {
		return err
	}

	text := fmt.Sprintf("Welcome, %s.", claimItem("name"))
	return th.seesElementWithText(`html body h1`, text)
}

type waitFor func() error

func (th *TestHarness) fillsInFormValue(selector, value string, waitForForm waitFor) error {
	if err := waitForForm(); err != nil {
		return err
	}

	if err := th.entersText(selector, value); err != nil {
		return err
	}

	return nil
}

func (th *TestHarness) clicksFormCheckItem(selector string, waitForForm waitFor) error {
	if err := waitForForm(); err != nil {
		return err
	}

	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return false, nil
		}

		if err = elem.Click(); err != nil {
			return false, err
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) fillsInUsername() error {
	return th.fillsInFormValue(`input[name="identifier"]`, os.Getenv("OKTA_IDX_USER_NAME"), th.waitForLoginForm)
}

func (th *TestHarness) fillsInIncorrectUsername() error {
	return th.fillsInFormValue(`input[name="identifier"]`, "TYPO"+os.Getenv("OKTA_IDX_USER_NAME"), th.waitForLoginForm)
}

func (th *TestHarness) fillsInPassword() error {
	return th.fillsInFormValue(`input[name="password"]`, os.Getenv("OKTA_IDX_PASSWORD"), th.waitForLoginForm)
}

func (th *TestHarness) fillsInIncorrectPassword() error {
	return th.fillsInFormValue(`input[name="password"]`, "wrong password", th.waitForLoginForm)
}

func (th *TestHarness) fillsInSignUpFirstName() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="firstName"]`, th.currentProfile.GivenName, th.waitForRegistrationForm)
}

func (th *TestHarness) fillsInSignUpLastName() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="lastName"]`, th.currentProfile.FamilyName, th.waitForRegistrationForm)
}

func (th *TestHarness) fillsInSignUpEmail() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="email"]`, th.currentProfile.EmailAddress, th.waitForRegistrationForm)
}
func (th *TestHarness) fillsInInvalidSignUpEmail() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="email"]`, "invalid-email-address-dot-com", th.waitForRegistrationForm)
}

func (th *TestHarness) fillsInSignUpPassword() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="newPassword"]`, th.currentProfile.Password, th.waitForEnrollPasswordForm)
}

func (th *TestHarness) fillsInSignUpConfirmPassword() error {
	if th.currentProfile == nil {
		return errors.New("test harness doesn't have a current profile")
	}
	return th.fillsInFormValue(`input[name="confirmPassword"]`, th.currentProfile.Password, th.waitForEnrollPasswordForm)
}

func (th *TestHarness) submitsNewPasswordForm() error {
	return th.clicksButtonWithText(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) matchErrorMessage(partialErrStr string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, ERROR_DIV)
		if err != nil {
			return false, nil
		}

		text, err := elem.Text()
		if err != nil {
			return false, nil
		}

		if matched, _ := regexp.MatchString(partialErrStr, text); !matched {
			return false, fmt.Errorf("expected error message %q to match %q", text, partialErrStr)
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) seesAuthFailedErrorMessage() error {
	return th.matchErrorMessage("Authentication failed")
}

func (th *TestHarness) seesNoAccountErrorMessage() error {
	return th.matchErrorMessage("There is no account with the Username")
}

func (th *TestHarness) seesErrorMessage(message string) error {
	return th.matchErrorMessage(message)
}

func (th *TestHarness) isLoggedOut() error {

	text := fmt.Sprintf("Welcome, %s.", claimItem("name"))
	return th.doesNotSeeElementWithText(`html body h1`, text)
}

func (th *TestHarness) seesClaimsTable() error {
	claims := claims()

	for claim, value := range claims {
		keyID := fmt.Sprintf("%s-key", claim)
		err := th.seesElementIDWithValue(keyID, claim)
		if err != nil {
			return err
		}

		valID := fmt.Sprintf("%s-value", claim)
		if err = th.seesElementIDWithValue(valID, value); err != nil {
			return err
		}
	}

	return nil
}

func (th *TestHarness) doesntSeeClaimsTable() error {
	claims := claims()

	for claim, value := range claims {
		keyID := fmt.Sprintf("%s-key", claim)
		err := th.doesntSeeElementIDWithValue(keyID, claim)
		if err != nil {
			return err
		}

		valID := fmt.Sprintf("%s-value", claim)
		if err = th.doesntSeeElementIDWithValue(valID, value); err != nil {
			return err
		}
	}

	return nil
}

func (th *TestHarness) seesLogoutButton() error {
	return th.seesElementWithText(`button[type="submit"]`, "Logout")
}

func (th *TestHarness) clicksLogoutButton() error {
	return th.clicksButtonWithText(`button[type="submit"]`, "Logout")
}

func (th *TestHarness) clicksForgotPasswordButton() error {
	return th.clickLink("Forgot your password?")
}

func (th *TestHarness) seesElement(selector string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		if _, err := th.wd.FindElement(selenium.ByCSSSelector, selector); err != nil {
			return false, nil
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) clickLink(text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByLinkText, text)
		if err != nil {
			return false, nil
		}

		if err = elem.Click(); err != nil {
			return false, err
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) entersText(selector, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return false, nil
		}

		if err = elem.Clear(); err != nil {
			return false, err
		}

		if err = elem.SendKeys(text); err != nil {
			return false, err
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) seesElementWithText(selector, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return false, nil
		}

		elemText, err := elem.Text()
		if err != nil {
			return false, nil
		}

		if strings.TrimSpace(elemText) != text {
			return false, nil
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) doesNotSeeElementWithText(selector, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return true, nil
		}

		elemText, err := elem.Text()
		if err != nil {
			return true, nil
		}
		if strings.TrimSpace(elemText) != text {
			return true, nil
		}

		return false, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) clicksButtonWithText(selector, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return false, nil
		}

		elemText, err := elem.Text()
		if err != nil {
			return false, nil
		}

		if strings.TrimSpace(elemText) != text {
			return false, nil
		}

		if err = elem.Click(); err != nil {
			return false, err
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}
func (th *TestHarness) clicksInputWithValue(selector, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return false, nil
		}

		value, err := elem.GetAttribute("value")
		if err != nil {
			return false, nil
		}

		if strings.TrimSpace(value) != text {
			return false, nil
		}

		if err = elem.Click(); err != nil {
			return false, err
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) seesElementIDWithValue(elementID, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elem, err := th.wd.FindElement(selenium.ByID, elementID)
		if err != nil {
			return false, nil
		}

		elemText, err := elem.Text()
		if err != nil {
			return false, nil
		}

		if strings.TrimSpace(elemText) != text {
			return false, nil
		}

		return true, nil
	}, defaultTimeout(), defaultInterval())

	return err
}

func (th *TestHarness) doesntSeeElementIDWithValue(elementID, text string) error {
	err := th.wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		elems, err := th.wd.FindElements(selenium.ByID, text)
		if err != nil {
			return false, nil
		}
		if len(elems) != 0 {
			return false, fmt.Errorf("didn't expect to find element id %q with text %q in page but found %d elems", elementID, text, len(elems))
		}

		return true, nil
	}, time.Duration(time.Millisecond*50), time.Duration(time.Millisecond*50))

	if err == nil {
		return nil
	}

	if matched, _ := regexp.MatchString("timeout", err.Error()); matched {
		return nil
	}

	return err
}

func (th *TestHarness) noop() error {
	return nil
}

func (th *TestHarness) navigatesToThePasswordRecoveryView() error {
	rootURL := fmt.Sprintf("http://%s/passwordRecovery", th.server.Address())
	err := th.wd.Get(rootURL)
	if err != nil {
		return err
	}
	return th.waitForPageRender()
}

func (th *TestHarness) inputsCorrectEmail() error {
	err := th.entersText(`input[name="identifier"]`, os.Getenv("OKTA_IDX_USER_NAME_RESET"))
	if err != nil {
		return err
	}
	return nil
}

func (th *TestHarness) submitsForm(selector, text string) error {
	if err := th.clicksButtonWithText(selector, text); err != nil {
		return err
	}

	return nil
}

func (th *TestHarness) submitsLoginForm() error {
	return th.submitsForm(`button[type="submit"]`, "Login")
}

func (th *TestHarness) submitsTheRecoveryForm() error {
	return th.submitsForm(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) submitsRegistrationForm() error {
	return th.submitsForm(`button[type="submit"]`, "Register")
}

func (th *TestHarness) submitsTheCodeForm() error {
	return th.submitsForm(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) submitsNewPassword() error {
	return th.submitsForm(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) seesPageToInputTheCode() error {
	return th.seesElement(`form[action="/passwordRecovery/code"]`)
}

func (th *TestHarness) fillsInTheCorrectCode() error {
	profileURL, err := th.profileURL()
	if err != nil {
		return err
	}
	code, err := th.verificationCode(profileURL, EMAIL_CODE_TYPE)
	if err != nil {
		return fmt.Errorf("faild to find latest verification code for user %s: %v", os.Getenv("OKTA_IDX_USER_NAME_RESET"), err)
	}
	if err = th.entersText(`input[name="code"]`, code); err != nil {
		return err
	}
	return nil
}

func (th *TestHarness) seesPageToSetNewPassword() error {
	return th.seesElement(`form[action="/passwordRecovery/newPassword"]`)
}

func (th *TestHarness) fillsPassword() error {
	p := randomString()
	if err := th.entersText(`input[name="newPassword"]`, p); err != nil {
		return err
	}
	if err := th.entersText(`input[name="confirmPassword"]`, p); err != nil {
		return err
	}
	return nil
}

func (th *TestHarness) inputsIncorrectEmail() error {
	randomString()
	err := th.entersText(`input[name="identifier"]`, strings.ReplaceAll(os.Getenv("OKTA_IDX_USER_NAME_RESET"), "@", "+1@"))
	if err != nil {
		return err
	}
	return nil
}

func (th *TestHarness) noAccountError(errorAcc string) error {
	errorAcc += " " + strings.ReplaceAll(os.Getenv("OKTA_IDX_USER_NAME_RESET"), "@", "+1@") + "."
	err := th.seesElementWithText(ERROR_DIV, errorAcc)
	if err != nil {
		return err
	}
	return nil
}

func (th *TestHarness) destroyCurrentProfile() error {
	if th.currentProfile == nil {
		return nil
	}

	err := th.deleteProfile(th.currentProfile)
	th.currentProfile = nil

	return err
}

func (th *TestHarness) createCurrentProfile(name string) error {
	profile, err := th.createProfile(name)
	th.currentProfile = profile

	return err
}

func (th *TestHarness) selectsEmailEnrollmentFactor() error {
	if err := th.clicksFormCheckItem(`input[name="push_email"]`, th.waitForEnrollFactorForm); err != nil {
		return err
	}

	return th.clicksButtonWithText(`button[type="submit"]`, "Continue")
}

func (th *TestHarness) selectsPhoneEnrollmentFactor() error {
	if err := th.clicksFormCheckItem(`input[name="push_phone"]`, th.waitForEnrollFactorForm); err != nil {
		return err
	}

	return th.clicksButtonWithText(`button[type="submit"]`, "Continue")
}

func (th *TestHarness) clicksSkip() error {
	return th.clicksInputWithValue(`input[type="submit"]`, "Skip")
}

func (th *TestHarness) fillsInTheEnrollmentCode() error {
	code, err := th.verificationCode(th.currentProfile.URL, EMAIL_CODE_TYPE)
	if err != nil {
		return fmt.Errorf("faild to find latest verification code for user %s: %v", th.currentProfile.ProfileID, err)
	}
	if err = th.entersText(`input[name="code"]`, code); err != nil {
		return err
	}
	return th.clicksButtonWithText(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) fillsInTheEnrollmentPhone() error {
	if err := th.entersText(`input[name="phoneNumber"]`, th.currentProfile.PhoneNumber); err != nil {
		return err
	}
	return th.clicksButtonWithText(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) fillsInReceiveSMSCode() error {
	if err := th.clicksFormCheckItem(`input[name="sms"]`, th.waitForEnrollPhoneMethodForm); err != nil {
		return err
	}

	return th.clicksButtonWithText(`button[type="submit"]`, "Continue")
}

func (th *TestHarness) fillsInTheEnrollmentCodeSMS() error {
	code, err := th.verificationCode(th.currentProfile.URL, SMS_CODE_TYPE)
	if err != nil {
		return fmt.Errorf("faild to find latest verification code for user %s: %v", th.currentProfile.ProfileID, err)
	}
	if err = th.entersText(`input[name="code"]`, code); err != nil {
		return err
	}

	return nil
}

func (th *TestHarness) debugSleep(amount string) error {
	// And sleep 60s
	d, err := time.ParseDuration(amount)
	if err != nil {
		return err
	}
	time.Sleep(d)
	return nil
}

func (th *TestHarness) clicksVerifySMSCode() error {
	return th.clicksButtonWithText(`button[type="submit"]`, "Submit")
}

func (th *TestHarness) verificationCode(profileURL, codeType string) (string, error) {
	checker := time.Tick(time.Second * 5)
	timeout := time.After(time.Minute)
loop:
	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("%s didn't receive %s verification code (one minute timeout)", profileURL, codeType)
		case <-checker:
			code, err := th.latestVerificationCode(profileURL, codeType)
			if err != nil {
				break loop
			}
			if code != "" {
				return code, nil
			}
		}
	}
	return "", fmt.Errorf("%s didn't receive %s verification code", profileURL, codeType)
}

func (th *TestHarness) latestVerificationCode(profileURL, codeType string) (string, error) {
	// codeType: email, sms, voice
	// e.g. api.a18n.help/v1/profile/nAfBjtIFF3/sms/latest
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/latest", profileURL, codeType), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", os.Getenv("A18N_API_KEY"))
	resp, err := th.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var content A18NContent
	err = json.Unmarshal(body, &content)
	if err != nil {
		return "", err
	}
	if time.Now().UTC().Sub(content.CreatedAt.UTC()) < time.Second*30 {
		verificationCodeRegexp := regexp.MustCompile(`[:\s][0-9]{6}`)
		code := verificationCodeRegexp.FindString(content.Content)
		if code != "" {
			return strings.TrimSpace(code), nil
		}
	}
	return "", nil
}

func (th *TestHarness) deleteProfile(profile *A18NProfile) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/profile/%s", a18nApiURL(), profile.ProfileID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", os.Getenv("A18N_API_KEY"))
	resp, err := th.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (th *TestHarness) createProfile(name string) (*A18NProfile, error) {
	data := fmt.Sprintf("{\"displayName\":%q}", name)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/profile", a18nApiURL()), bytes.NewBufferString(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", os.Getenv("A18N_API_KEY"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data)))
	resp, err := th.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var profile A18NProfile
	err = json.Unmarshal(body, &profile)
	if err != nil {
		return nil, err
	}

	givenFamily := strings.Split(name, " ")
	profile.GivenName = givenFamily[0]
	profile.FamilyName = givenFamily[1]
	profile.Password = randomString()

	return &profile, nil
}

func (th *TestHarness) profiles() (*A18NProfiles, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/profile", a18nApiURL()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", os.Getenv("A18N_API_KEY"))
	resp, err := th.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var profiles A18NProfiles
	err = json.Unmarshal(body, &profiles)
	if err != nil {
		return nil, err
	}
	return &profiles, nil
}

func (th *TestHarness) profileURL() (string, error) {
	profiles, err := th.profiles()
	if err != nil {
		return "", err
	}
	userName := os.Getenv("OKTA_IDX_USER_NAME_RESET")
	for _, v := range profiles.Profiles {
		if v.EmailAddress == userName {
			return v.URL, nil
		}
	}
	return "", fmt.Errorf("profile with %s doesn't exist if REST API for receiving MFA verification codes", userName)
}

func (th *TestHarness) testProfile() (*A18NProfile, error) {
	profiles, err := th.profiles()
	if err != nil {
		return nil, err
	}
	displayName := "golang-idx-sdk"
	for _, profile := range profiles.Profiles {
		if profile.DisplayName == displayName {
			return &profile, nil
		}
	}
	return nil, fmt.Errorf("profile with %s doesn't exist if REST API for receiving MFA verification codes", displayName)
}
