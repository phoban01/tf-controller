package controllers

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
	. "github.com/onsi/gomega"

	infrav1 "github.com/chanwit/tf-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:docs-gen:collapse=Imports

func Test_000074_varsfrom_accepts_many_secrets_with_last_supplied_key_precedence(t *testing.T) {
	const (
		sourceName        = "src-vars-from-many-config-maps"
		terraformName     = "tf-vars-from-many-config-maps"
		generatedVarsFile = "generated.auto.tfvars.json"
	)

	g := NewWithT(t)
	ctx := context.Background()

	// By("setting up some variables")
	secretData := []struct {
		name string
		data map[string]string
	}{
		{
			name: "config-map-1",
			data: map[string]string{
				"key-1": "value-1",
				"key-2": "value-2",
			},
		},
		{
			name: "config-map-2",
			data: map[string]string{
				"key-3": "value-3",
				"key-1": "value-4",
			},
		},
	}

	By("create the secrets")
	for _, secret := range secretData {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.name,
				Namespace: "flux-system",
			},
			StringData: secret.data,
		}
		g.Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
	}

	By("creating a temporary working directory")
	workDir, err := os.MkdirTemp("", terraformName+"*")
	g.Expect(err).Should(BeNil())

	By("looking up the path of the terraform binary")
	execPath, err := exec.LookPath("terraform")
	g.Expect(err).Should(BeNil())

	By("creating a new TF exec instance")
	tfExec, err := tfexec.NewTerraform(workDir, execPath)
	g.Expect(err).Should(BeNil())

	By("creating a new TF resource with slice of ConfigMaps")
	var varsRef []infrav1.VarsReference
	for _, cm := range secretData {
		varsRef = append(varsRef, infrav1.VarsReference{
			Kind: "Secret",
			Name: cm.name,
		})
	}
	terraform := infrav1.Terraform{
		ObjectMeta: metav1.ObjectMeta{
			Name:      terraformName,
			Namespace: "flux-system",
		},
		Spec: infrav1.TerraformSpec{
			VarsFrom: varsRef,
		},
	}

	_, err = reconciler.generateVarsForTF(ctx, terraform, tfExec, "main")
	g.Expect(err).Should(BeNil())

	By("verifying the generated vars file matches the expected result")
	varsFilePath := filepath.Join(tfExec.WorkingDir(), generatedVarsFile)

	// read vars file
	data, err := os.ReadFile(varsFilePath)
	g.Expect(err).Should(BeNil())

	// unmarshal data
	var vars map[string]string
	g.Expect(json.Unmarshal(data, &vars)).Should(Succeed())

	// check vars
	expectedResult := map[string]string{
		"key-1": "value-4",
		"key-2": "value-2",
		"key-3": "value-3",
	}
	g.Expect(vars).Should(Equal(expectedResult))
}
