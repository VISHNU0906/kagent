package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kagent-dev/kagent/go/api/v1alpha3"
)

func TestE2EUnsupportedReferenceKinds(t *testing.T) {
	interactionTarget(t)
	kube := interactionKubeClient(t)

	t.Run("Given model and egress policy references to kinds kagent does not resolve, when an Agent is created, assert that both conditions report InvalidKind", func(t *testing.T) {
		agent := &v1alpha3.Agent{ObjectMeta: metav1.ObjectMeta{GenerateName: "unsupported-kinds-", Namespace: "kagent"}, Spec: v1alpha3.AgentSpec{
			HarnessRef:       corev1.LocalObjectReference{Name: "kagent"},
			EgressPolicyRefs: []v1alpha3.EgressPolicyReference{{Group: new("example.com"), Kind: new("Policy"), Name: "tickets"}},
			Template: v1alpha3.AgentTemplateSource{Inline: &v1alpha3.AgentTemplateSpec{
				ModelConfig:  &v1alpha3.ModelConfigReference{Group: new("example.com"), Kind: new("Model"), Name: "default-model-config"},
				SystemPrompt: "Reply briefly.",
			}},
		}}
		require.NoError(t, kube.Create(t.Context(), agent))
		t.Cleanup(func() { require.NoError(t, kube.Delete(context.Background(), agent)) })

		// Wait for both conditions to describe this generation, then assert their outcome.
		err := wait.PollUntilContextTimeout(t.Context(), time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
			if err := kube.Get(ctx, ctrlclient.ObjectKeyFromObject(agent), agent); err != nil {
				return false, err
			}
			for _, conditionType := range []string{v1alpha3.AgentConditionResolvedRefs, v1alpha3.AgentConditionEgressPoliciesApplied} {
				condition := meta.FindStatusCondition(agent.Status.Conditions, conditionType)
				if condition == nil || condition.ObservedGeneration != agent.Generation || condition.Status == metav1.ConditionUnknown {
					return false, nil
				}
			}
			return true, nil
		})
		require.NoError(t, err, "last conditions: %+v", agent.Status.Conditions)
		for _, conditionType := range []string{v1alpha3.AgentConditionResolvedRefs, v1alpha3.AgentConditionEgressPoliciesApplied} {
			condition := meta.FindStatusCondition(agent.Status.Conditions, conditionType)
			require.Equal(t, metav1.ConditionFalse, condition.Status, "%s: %s", conditionType, condition.Message)
			require.Equal(t, "InvalidKind", condition.Reason, "%s: %s", conditionType, condition.Message)
		}
		require.Empty(t, agent.Status.DesiredRevision)
	})
}
