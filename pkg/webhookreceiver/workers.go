package webhookreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openshift-online/ocm-cli/pkg/arguments"
	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/ocm"
	"github.com/openshift/ocm-agent/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"time"
)

const (
	TemplateAnnotation = "template"
)

func processAMReceiver(d AMReceiverData) {
	log.WithField("AMReceiverData", d).Info("Process alert data")

	for _, alert := range d.Alerts.Firing() {
		if template, ok := alert.Annotations[TemplateAnnotation]; ok {

			log.WithField("template", template).Info("would send an SL for this template")
			err := sendSLForTemplate(template)
			if err != nil {
				log.WithError(err).Error("well that aint good")
			}

		} else {
			log.WithField("alert", alert).Info("nothing to do for this alert")
		}
	}
}

func sendSLForTemplate(t string) error {
	c, err := util.BuildDynamicClient()
	if err != nil {
		return err
	}

	// First see if there's a CR for the template
	gvr := schema.GroupVersionResource{
		Group:    "ocmagent.managed.openshift.io",
		Version:  "v1alpha1",
		Resource: "managednotifications",
	}
	oa, err := c.Resource(gvr).Namespace("openshift-ocm-agent-operator").Get(context.TODO(), t, metav1.GetOptions{})
	if err != nil {
		return err
	}
	uobj := oa.UnstructuredContent()
	uobjSpec := uobj["spec"].(map[string]interface{})
	if _, ok := uobjSpec["template"]; !ok {
		return fmt.Errorf("no template CR found")
	}
	template := fmt.Sprintf("%v", uobjSpec["template"])

	if _, ok := uobj["status"]; ok {
		// We have a status, when was the last time we sent?
		uobjStatus := uobj["status"].(map[string]interface{})
		if uobjStatus != nil {
			if _, ok := uobjStatus["lastSent"]; ok {
				lastSent := fmt.Sprintf("%v", uobjStatus["lastSent"])
				ts, _ := time.Parse("2006-01-02T15:04:05Z", lastSent)
				if time.Now().Sub(ts) < (30 * time.Minute) {
					log.Info("I sent a SL in the last 30 minutes, not going to send again")
					return nil
				}
			}
		}
	}

	log.WithField("template", template).Info("id send an SL except this isnt written yet")
	err = sendSL(template)
	if err != nil {
		return err
	}

	// Update lastSent timestamp
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oa, err := c.Resource(gvr).Namespace("openshift-ocm-agent-operator").Get(context.TODO(), t, metav1.GetOptions{})
		uobj := oa.UnstructuredContent()
		uobj["status"] = map[string]interface{}{
			"lastSent": &metav1.Time{Time: time.Now()},
		}
		var uOaObj2 unstructured.Unstructured
		uOaObj2.SetUnstructuredContent(uobj)
		_, err = c.Resource(gvr).Namespace("openshift-ocm-agent-operator").UpdateStatus(context.TODO(), &uOaObj2, metav1.UpdateOptions{})

		return err
	})
	return err
}

func sendSL(t string) error {
	// good to send a SL

	// hard codey
	clusterId := "66fbb88f-9267-4e26-b6ef-62ad9c4af310"

	ocmconn, err := ocm.NewConnection().Build(viper.GetString(config.OcmUrl),
		clusterId,
		viper.GetString(config.AccessToken))
	if err != nil {
		return err
	}

	req := ocmconn.Post()
	err = arguments.ApplyPathArg(req, "/api/service_logs/v1/cluster_logs")
	if err != nil {
		return err
	}

	sl := ocm.ServiceLog{
		ServiceName:  "SREManualAction",
		ClusterUUID:  clusterId,
		Summary:      "I should have added a summary to the managed notification CRD",
		Description:  t,
		InternalOnly: false,
	}
	slAsBytes, err := json.Marshal(sl)
	if err != nil {
		return err
	}

	req.Bytes(slAsBytes)
	_, err = req.Send()
	if err != nil {
		return err
	}

	log.Info("we sent the service log.. maybe????")
	return nil
}
