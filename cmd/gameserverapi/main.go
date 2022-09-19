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
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/playfab/thundernetes/cmd/gameserverapi/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
	LabelBuildName            = "BuildName"
)

// @title          GameServer API
// @version        1.0
// @description    This is a service for managing GameServer and GameServerBuilds
// @termsOfService http://swagger.io/terms/
// @license.name   Apache 2.0
// @license.url    http://www.apache.org/licenses/LICENSE-2.0.html
func main() {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mpsv1alpha1.AddToScheme(scheme)

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	// creating a split client https://cs.github.com/kubernetes-sigs/controller-runtime/blob/eb39b8eb28cfe920fa2450eb38f814fc9e8003e8/pkg/cluster/cluster.go#L265
	// to facilitate reads from the cache and writes with the live API client
	ctx := context.Background()
	config := ctrl.GetConfigOrDie()
	ca, err := cache.New(config, cache.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Starting cache")
	go func() {
		err := ca.Start(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()

	b := ca.WaitForCacheSync(ctx)
	if !b {
		log.Fatal("Cache sync failed")
	}

	log.Info("Cache sync succeeded")
	kubeClient, err = cluster.DefaultNewClient(ca, config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	r := setupRouter()
	// By default it serves on :5001 unless a
	// PORT environment variable was defined.
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = fmt.Sprintf(":%d", listeningPort)
	}
	r.Run(addr)
}

func setSwaggerInfo(c *gin.Context) {
	// dynamically sets swagger host and base path
	docs.SwaggerInfo.Host = c.Request.Host
	docs.SwaggerInfo.BasePath = urlprefix
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		setSwaggerInfo(c)
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PATCH, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Max")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(CORSMiddleware())
	r.POST(fmt.Sprintf("%s/gameserverbuilds", urlprefix), createGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds", urlprefix), listGameServeBuilds)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), getGameServerBuild)
	r.DELETE(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), deleteGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName/gameservers", urlprefix), listGameServersForBuild)
	r.GET(fmt.Sprintf("%s/gameservers", urlprefix), listGameServers)
	r.GET(fmt.Sprintf("%s/gameservers/:namespace/:gameServerName", urlprefix), getGameServer)
	r.DELETE(fmt.Sprintf("%s/gameservers/:namespace/:gameServerName", urlprefix), deleteGameServer)
	r.PATCH(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName", urlprefix), patchGameServerBuild)
	r.GET(fmt.Sprintf("%s/gameserverbuilds/:namespace/:buildName/gameserverdetails", urlprefix), listGameServerDetailsForBuild)
	r.GET(fmt.Sprintf("%s/gameserverdetails/:namespace/:gameServerDetailName", urlprefix), getGameServerDetail)
	r.GET("/healthz", healthz)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return r
}

// @Summary Create a GameServerBuild
// @ID create-gameserverbuild
// @Produce json
// @Param data body mpsv1alpha1.GameServerBuild true "gsb"
// @Success 201 {object} mpsv1alpha1.GameServerBuild
// @Failure 500 {object} error
// @Router /gameserverbuilds [post]
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

// @Summary get list of GameServerBuilds
// @ID get-list-gameserverbuilds
// @Produce json
// @Success 200 {object} mpsv1alpha1.GameServerBuildList
// @Failure 500 {object} error
// @Router /gameserverbuilds [get]
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

// @Summary get a GameServerBuild by buildName and namespace
// @ID get-game-server-build-by-buildName&namespace
// @Produce json
// @Param namespace path string true "namespaceParam"
// @Param buildName path string true "buildNameParam"
// @Success 200 {object} mpsv1alpha1.GameServerBuild
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameserverbuilds/{namespace}/{buildName} [get]
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

// @Summary get list of GameServers
// @ID get-list-gameservers
// @Produce json
// @Success 200 {object} mpsv1alpha1.GameServerList
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameservers [get]
func listGameServers(c *gin.Context) {
	var gsList mpsv1alpha1.GameServerList
	err := kubeClient.List(ctx, &gsList)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gsList)
	}
}

// @Summary get list of GameServers for a given build
// @ID get-list-gameservers-by-build
// @Produce json
// @Param namespace path string true "buildNameParam"
// @Success 200 {object} mpsv1alpha1.GameServerList
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameservers/{buildNameParam} [get]
func listGameServersForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsList mpsv1alpha1.GameServerList
	err := kubeClient.List(ctx, &gsList, client.MatchingLabels{LabelBuildName: buildName})
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

// @Summary get GameServer by GameServerName and namespace
// @ID get-gameserver-by-gameservername-and-namespace
// @Produce json
// @Param namespace path string true "gameServerNameParam"
// @Param namespace path string true "namespaceParam"
// @Success 200 {object} mpsv1alpha1.GameServer
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameservers/{gameServerNameParam}/{namespaceParam} [get]
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

// @Summary delete GameServer by GameServerName and namespace
// @ID delete-gameserver-by-gameservername-and-namespace
// @Produce json
// @Param namespace path string true "gameServerNameParam"
// @Param namespace path string true "namespaceParam"
// @Success 200 {object} map[string]string
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameservers/{gameServerNameParam}/{namespaceParam} [delete]
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

// @Summary patch GameServerBuild by buildName and namespace
// @ID path-gameserverbuild-by-buildname-and-namespace
// @Produce json
// @Param namespace path string true "buildNameParam"
// @Param namespace path string true "namespaceParam"
// @Param data body map[string]interface{} true "gsbMap"
// @Success 200 {object} mpsv1alpha1.GameServerBuild
// @Failure 400 {object} error
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameserverbuilds/{buildNameParam}/{namespaceParam} [patch]
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

// @Summary delete GameServerBuild by buildName and namespace
// @ID path-gameserverbuild-by-buildname-and-namespace
// @Produce json
// @Param namespace path string true "buildNameParam"
// @Param namespace path string true "namespaceParam"
// @Success 200 {object} map[string]string
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameserverbuilds/{buildNameParam}/{namespaceParam} [delete]
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

// @Summary get GameServerDetailList by buildName
// @ID get-gameserver-details-by-build
// @Produce json
// @Param namespace path string true "buildNameParam"
// @Success 200 {object} mpsv1alpha1.GameServerDetailList
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameserverbuilds/{buildNameParam} [get]
func listGameServerDetailsForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsdList mpsv1alpha1.GameServerDetailList
	err := kubeClient.List(ctx, &gsdList, client.MatchingLabels{LabelBuildName: buildName})
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

// @Summary get GameServerDetail by GameServerDetailName and namespace
// @ID get-gameserver-details-by-gameserverdetailname-and-namespace
// @Produce json
// @Param namespace path string true "gameServerDetailNameParam"
// @Success 200 {object} mpsv1alpha1.GameServerDetail
// @Failure 404 {object} error
// @Failure 500 {object} error
// @Router /gameserverbuilds/{namespace}/{gameServerDetailNameParam} [get]
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
