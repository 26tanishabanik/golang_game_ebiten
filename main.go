package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/google/uuid"
)

const (
	screenWidth  = 1280
	screenHeight = 640
	balloonSpeed = 7
	bulletSpeed  = 15
)

type Balloon struct {
	x, y    float64
	shooted int
	id      string
	active  bool
}

type Bullet struct {
	x, y   float64
	active bool
}

type Game struct {
	balloons []*Balloon
	bullets  []*Bullet
	score    int
}

var (
	balloonImg *ebiten.Image
	bulletImg  *ebiten.Image
	idMap []string
	nodeList  []string
)

type Node struct {
	Name string
	Status  string 
	Roles   string           
	Age     string   
	Version  string
}

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}

func getKubeConfig() *string{
	var kubeConfig *string
	if home := HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube","config"), "")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "")
	}
	return kubeConfig
}

func ClientSetup() (*kubernetes.Clientset, error) {
	var kubeConfig *string
	if flag.Lookup("kubeconfig") == nil {
		kubeConfig = getKubeConfig()
		config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
		if err != nil {
			fmt.Printf("Error in new client config: %s\n", err)
			return nil,err
		}
		clientset := kubernetes.NewForConfigOrDie(config)
		return clientset, nil
	}
	
	config, err := clientcmd.BuildConfigFromFlags("", flag.Lookup("kubeconfig").Value.String())
	if err != nil {
		fmt.Printf("Error in new client config: %s\n", err)
		return nil,err
	}
	clientset := kubernetes.NewForConfigOrDie(config)
	return clientset, nil

}

func GetNodeNames() ([]string, error) {
	clientSet, err := ClientSetup()
	if err != nil {
		fmt.Println("error in creating client set: ", err)
		return nil, err
	}
	
	nodes, err := clientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("error in getting the list of nodes: ", err)
		return nil, err
	}
	for  _, n := range nodes.Items {
		nodeList = append(nodeList, n.Name)
	}
	return nodeList, nil
}

func CreatePod(nodeName string) error{
	id := uuid.New().String()
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s",id),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: "OnFailure",
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
			NodeName: nodeName,
		},
	}
	clientSet, err := ClientSetup()
	if err != nil {
		fmt.Println("error in creating client set: ", err)
		return err
	}
	if _, err := clientSet.CoreV1().Pods("default").Create(context.Background(), &pod, metav1.CreateOptions{}); err != nil {
		fmt.Printf("error in creating pod on node %s\n: ", nodeName)
		return err
	}
	return nil
}

func (g *Game) Update() error {
	nodeList, err := GetNodeNames()
	if err != nil {
		fmt.Println("error in getting node list: ", err)
		os.Exit(1)
	}
	
	for _, b := range g.balloons {
		if b.active {
			b.y -= balloonSpeed
		}
	}

	for i := 0; i < len(g.bullets); i++ {
		g.bullets[i].y -= bulletSpeed
		if g.bullets[i].y < 0 {
			g.bullets = append(g.bullets[:i], g.bullets[i+1:]...)
			i--
		}
	}

	
	for _, b := range g.bullets {
		for _, b2 := range g.balloons {
			if b2.active && b.x >= b2.x && b.x <= b2.x+50 && b.y <= b2.y+50 {
				b.active = false
				b2.active = false
				b2.shooted = 1
				b2.id = nodeList[rand.Intn(len(nodeList))]
				if err := CreatePod(b2.id); err != nil {
					fmt.Printf("error in creating pod on the node %s\n: %v", b2.id, err)
					os.Exit(1)
				}
				g.score++
			}
		}
	}

	
	if rand.Float64() < 0.05 {
		for _, b := range g.balloons {
			if !b.active {
				b.active = true
				b.x = rand.Float64() * (screenWidth - 50)
				b.y = screenHeight
				break
			}
		}
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.bullets = append(g.bullets, &Bullet{x: float64(x), y: float64(y)})
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	
	for _, bullet := range g.bullets {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(bullet.x, bullet.y)
		screen.DrawImage(bulletImg, op)
	}

	for _, balloon := range g.balloons {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(balloon.x, balloon.y)
		screen.DrawImage(balloonImg, op)
		
	}

	for _, balloon := range g.balloons {
		if balloon.shooted == 1 {
			idMap = append(idMap, balloon.id)
		}
		balloon.shooted = 0
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("Score: %d", g.score))
	if len(idMap) > 0 {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Pods scheduled on node %s", idMap[len(idMap)-1]), 0, 50)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Balloon Shooter")

	var err error
	balloonImg, _, err = ebitenutil.NewImageFromFile("balloon.png")
	if err != nil {
		log.Fatal(err)
	}

	bulletImg, _, err = ebitenutil.NewImageFromFile("bullet.png")
	if err != nil {
		log.Fatal(err)
	}

	game := &Game{
		balloons: make([]*Balloon, 60),
		bullets:  make([]*Bullet, 60),
	}

	for i := 0; i < 60; i++ {
		game.balloons[i] = &Balloon{}
		game.bullets[i] = &Bullet{}
	}

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
