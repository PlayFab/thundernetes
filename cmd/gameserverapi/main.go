package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"net/http"

	"github.com/gin-gonic/gin"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeClient client.Client
	ctx        = context.Background()
)

const (
	buildNameParam            = "buildName"
	gameServerNameParam       = "gameServerName"
	namespaceParam            = "namespace"
	gameServerDetailNameParam = "gameServerDetailName"
	urlprefix                 = "/api/v1"
	listeningPort             = 5001
)

func main() {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mpsv1alpha1.AddToScheme(scheme)

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	var err error
	kubeClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	r := setupRouter()
	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = fmt.Sprintf(":%d", listeningPort)
	}
	r.Run(addr)
}

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.POST(fmt.Sprintf("%s/gameserverbuilds", urlprefix), createGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds", urlprefix), listGameServeBuilds)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), getGameServerBuild)
	r.DELETE(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), deleteGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName/gameservers", urlprefix), listGameServersForBuild)
	r.GET(fmt.Sprintf("%s/gameservers/:namespace/:gameServerName", urlprefix), getGameServer)
	r.DELETE(fmt.Sprintf("%s/gameservers/:namespace/:gameServerName", urlprefix), deleteGameServer)
	r.PATCH(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), patchGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName/gameserverdetails", urlprefix), listGameServerDetailsForBuild)
	r.GET(fmt.Sprintf("%s/gameserverdetails/:namespace/:gameServerDetailName", urlprefix), getGameServerDetail)
	r.GET("/healthz", healthz)
	return r
}

func createGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	err := c.BindJSON(&gsb)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = kubeClient.Create(ctx, &gsb)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gsb)
}

func listGameServeBuilds(c *gin.Context) {
	var gsbList mpsv1alpha1.GameServerBuildList
	err := kubeClient.List(ctx, &gsbList)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gsbList)
	}
}

func getGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	namespace := c.Param(namespaceParam)
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsb)
	}
}

func listGameServersForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsList mpsv1alpha1.GameServerList
	err := kubeClient.List(ctx, &gsList, client.MatchingLabels{"BuildName": buildName})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsList)
	}
}

func getGameServer(c *gin.Context) {
	gameServerName := c.Param(gameServerNameParam)
	namespace := c.Param(namespaceParam)
	var gs mpsv1alpha1.GameServer
	err := kubeClient.Get(ctx, client.ObjectKey{Name: gameServerName, Namespace: namespace}, &gs)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gs)
	}
}

func deleteGameServer(c *gin.Context) {
	gameServerName := c.Param(gameServerNameParam)
	namespace := c.Param(namespaceParam)
	var gs mpsv1alpha1.GameServer
	gs.Name = gameServerName
	gs.Namespace = namespace
	err := kubeClient.Delete(ctx, &gs)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Game server deleted"})
	}

}

func patchGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	namespace := c.Param(namespaceParam)
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	var m map[string]interface{}
	err = json.NewDecoder(c.Request.Body).Decode(&m)
	defer c.Request.Body.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newStandingByInterface := m["standingBy"]
	newMaxInterface := m["max"]

	// values on the map seem to be unmarshaled as float64
	newStandingBy, ok := newStandingByInterface.(float64)
	if !ok {
		log.Info("standingBy not a number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "standingBy not a number"})
		return
	}
	newMax, ok := newMaxInterface.(float64)
	if !ok {
		log.Info("max not a number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "max not a number"})
		return
	}
	if newStandingBy > newMax {
		log.Info("standingBy > max")
		c.JSON(http.StatusBadRequest, gin.H{"error": "standingBy > max"})
		return
	}

	patch := client.MergeFrom(gsb.DeepCopy())
	gsb.Spec.Max = int(newMax)
	gsb.Spec.StandingBy = int(newStandingBy)
	err = kubeClient.Patch(ctx, &gsb, patch)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gsb)
	}
}

func deleteGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	namespace := c.Param(namespaceParam)
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	err = kubeClient.Delete(ctx, &gsb)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Game server build deleted"})
	}
}

func listGameServerDetailsForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsdList mpsv1alpha1.GameServerDetailList
	err := kubeClient.List(ctx, &gsdList, client.MatchingLabels{"BuildName": buildName})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsdList)
	}
}

func getGameServerDetail(c *gin.Context) {
	gameServerDetailName := c.Param(gameServerDetailNameParam)
	namespace := c.Param(namespaceParam)
	var gsd mpsv1alpha1.GameServerDetail
	err := kubeClient.Get(ctx, client.ObjectKey{Name: gameServerDetailName, Namespace: namespace}, &gsd)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsd)
	}
}

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
