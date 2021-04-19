package alertmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	amSilence "github.com/prometheus/alertmanager/api/v2/client/silence"
	amv2Models "github.com/prometheus/alertmanager/api/v2/models"
)

//go:generate mockgen -destination=mocks/alertManagerSilenceClient.go -package=mocks github.com/openshift/managed-upgrade-operator/pkg/alertmanager AlertManagerSilencer
type AlertManagerSilencer interface {
	Create(matchers amv2Models.Matchers, startsAt strfmt.DateTime, endsAt strfmt.DateTime, creator string, comment string) error
	List(filter []string) (*amSilence.GetSilencesOK, error)
	Delete(id string) error
	Update(id string, endsAt strfmt.DateTime) error
	Filter(predicates ...SilencePredicate) (*[]amv2Models.GettableSilence, error)
}

type AlertManagerSilenceClient struct {
	Transport *httptransport.Runtime
}

// Creates a silence in Alertmanager instance defined in Transport
func (ams *AlertManagerSilenceClient) Create(matchers amv2Models.Matchers, startsAt strfmt.DateTime, endsAt strfmt.DateTime, creator string, comment string) error {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	pParams := &amSilence.PostSilencesParams{
		Silence: &amv2Models.PostableSilence{
			Silence: amv2Models.Silence{
				CreatedBy: &creator,
				Comment:   &comment,
				EndsAt:    &endsAt,
				StartsAt:  &startsAt,
				Matchers:  matchers,
			},
		},
		Context:    context.TODO(),
		HTTPClient: &http.Client{Transport: tr},
	}

	silenceClient := amSilence.New(ams.Transport, strfmt.Default)
	_, err := silenceClient.PostSilences(pParams)
	if err != nil {
		return err
	}

	return nil
}

// list silences in Alertmanager instance defined in Transport
func (ams *AlertManagerSilenceClient) List(filter []string) (*amSilence.GetSilencesOK, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	gParams := &amSilence.GetSilencesParams{
		Filter:     filter,
		Context:    context.TODO(),
		HTTPClient: &http.Client{Transport: tr},
	}

	silenceClient := amSilence.New(ams.Transport, strfmt.Default)
	results, err := silenceClient.GetSilences(gParams)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Delete silence in Alertmanager instance defined in Transport
func (ams *AlertManagerSilenceClient) Delete(id string) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	dParams := &amSilence.DeleteSilenceParams{
		SilenceID:  strfmt.UUID(id),
		Context:    context.TODO(),
		HTTPClient: &http.Client{Transport: tr},
	}

	silenceClient := amSilence.New(ams.Transport, strfmt.Default)
	_, err := silenceClient.DeleteSilence(dParams)
	if err != nil {
		return err
	}

	return nil
}

// Update silence end time in AlertManager instance defined in Transport
func (ams *AlertManagerSilenceClient) Update(id string, endsAt strfmt.DateTime) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	silenceClient := amSilence.New(ams.Transport, strfmt.Default)
	gParams := &amSilence.GetSilenceParams{
		SilenceID:  strfmt.UUID(id),
		Context:    context.TODO(),
		HTTPClient: &http.Client{Transport: tr},
	}
	result, err := silenceClient.GetSilence(gParams)
	if err != nil {
		return err
	}

	// Create a new silence first
	err = ams.Create(result.Payload.Matchers, *result.Payload.StartsAt, endsAt, *result.Payload.CreatedBy, *result.Payload.Comment)
	if err != nil {
		return fmt.Errorf("unable to create replacement silence: %v", err)
	}

	// Remove the old silence if it's still active
	if *result.Payload.Status.State == amv2Models.SilenceStatusStateActive {
		err = ams.Delete(*result.Payload.ID)
		if err != nil {
			return fmt.Errorf("unable to remove replaced silence: %v", err)
		}
	}

	return nil
}

type SilencePredicate func(*amv2Models.GettableSilence) bool

// Filter silences in Alertmanager based on the predicates
func (ams *AlertManagerSilenceClient) Filter(predicates ...SilencePredicate) (*[]amv2Models.GettableSilence, error) {
	silences, err := ams.List([]string{})
	if err != nil {
		return nil, err
	}

	filteredSilences := []amv2Models.GettableSilence{}
	for _, s := range silences.Payload {
		var match = true
		for _, p := range predicates {
			if !p(s) {
				match = false
				break
			}
		}
		if match {
			filteredSilences = append(filteredSilences, *s)
		}
	}

	return &filteredSilences, nil
}
