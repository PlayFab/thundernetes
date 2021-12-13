package main

import (
	"context"

	"github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type GameServerBuildInterface interface {
	Get(ctx context.Context, name string, options metav1.GetOptions) (*v1alpha1.GameServerBuild, error)
	Update(ctx context.Context, gameServerBuild *v1alpha1.GameServerBuild) (*v1alpha1.GameServerBuild, error)
}

type GameServerBuildClient struct {
	restClient rest.Interface
	ns         string
}

func (g *GameServerBuildClient) Get(ctx context.Context, name string) (*v1alpha1.GameServerBuild, error) {
	result := v1alpha1.GameServerBuild{}
	err := g.restClient.
		Get().
		Namespace(g.ns).
		Resource("gameserverbuilds").
		Name(name).
		Do(ctx).
		Into(&result)
	return &result, err
}

func (g *GameServerBuildClient) Update(ctx context.Context, gameServerBuild *v1alpha1.GameServerBuild) (*v1alpha1.GameServerBuild, error) {
	result := v1alpha1.GameServerBuild{}
	err := g.restClient.
		Put().
		Namespace(g.ns).
		Resource("gameserverbuilds").
		Name(gameServerBuild.Name).
		Body(gameServerBuild).
		Do(ctx).
		Into(&result)
	return &result, err
}
