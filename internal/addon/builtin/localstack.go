package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*LocalStack)(nil)

type LocalStack struct {
	addon.BaseAddon
}

func NewLocalStack(orch *orchestrator.Orchestrator, store storage.Store) *LocalStack {
	return &LocalStack{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (l *LocalStack) Name() string            { return "localstack" }
func (l *LocalStack) DisplayName() string      { return "LocalStack" }
func (l *LocalStack) Description() string      { return "AWS cloud service emulator (S3, KMS, STS, IAM, SSM, SQS, SNS, Lambda)" }
func (l *LocalStack) DefaultVersion() string   { return "4" }
func (l *LocalStack) AvailableVersions() []string { return []string{"4", "3"} }

func (l *LocalStack) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = l.DefaultVersion()
	}
	image := fmt.Sprintf("localstack/localstack:%s", version)

	services := "s3,sqs,sns,ssm,kms,sts,iam,secretsmanager,lambda"
	if v, ok := opts.Config["services"]; ok {
		services = v
	}
	debug := "0"
	if v, ok := opts.Config["debug"]; ok {
		debug = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "localstack-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "localstack",
				"torresmo.dev/addon": "localstack",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "localstack",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 4566, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "SERVICES", Value: services},
						{Name: "DEBUG", Value: debug},
					},
				},
			},
		},
	}

	if err := l.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating localstack pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "localstack",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "gateway", Port: 4566, TargetPort: 4566, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "localstack"},
		},
	}

	if err := l.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating localstack service: %w", err)
	}

	return nil
}

func (l *LocalStack) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = l.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "localstack")
	_ = l.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "localstack-0")
	return nil
}

func (l *LocalStack) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := l.Orchestrator().GetPod(ctx, clusterName, namespace, "localstack-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
