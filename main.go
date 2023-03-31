package main

import (
	// "bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	// "regexp"
	// "strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/google/uuid"
	// "github.com/hajimehoshi/ebiten/v2/text"
	// "golang.org/x/image/font"
	// "golang.org/x/image/font/opentype"
	// "github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	// "golang.org/x/image/font/basicfont"
	// "github.com/hajimehoshi/ebiten/inpututil"
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

func RunShellCommand(command string) (string, error) {
	var stdout bytes.Buffer
    var stderr bytes.Buffer
    cmd := exec.Command("bash", "-c", command)
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    err := cmd.Run()
	fmt.Println(stderr.String())
    return stdout.String(), err
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
	// output, err := RunShellCommand("kubectl get nodes")
	// if err != nil {
	// 	return nil, err
	// }
	// var columns = regexp.MustCompile(`\s+`)
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
	// r := strings.NewReader(output)
	// scanner := bufio.NewScanner(r)
	// for scanner.Scan() {
	// 	line := scanner.Text()
	// 	line = strings.TrimSpace(line)
	// 	cols := columns.Split(line, -1)
	// 	row := Node{cols[0], cols[1], cols[2], cols[3], cols[4]}
	// 	if row.Name != "NAME" {
	// 		nodeList = append(nodeList, row.Name)
	// 	}
	// }
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
	// Move active balloons
	for _, b := range g.balloons {
		if b.active {
			b.y -= balloonSpeed
		}
	}

	// Move active bullets
	// for _, b := range g.bullets {
	// 	if b.active {
	// 		b.y -= bulletSpeed
	// 	}
	// }
	for i := 0; i < len(g.bullets); i++ {
		g.bullets[i].y -= bulletSpeed
		if g.bullets[i].y < 0 {
			g.bullets = append(g.bullets[:i], g.bullets[i+1:]...)
			i--
		}
	}

	// Check for bullet hits
	for _, b := range g.bullets {
		// if b.active {
		// fmt.Println("bullet is active")
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
				
				// tt, err := opentype.Parse(fonts.MPlus1pRegular_ttf)
				// if err != nil {
				// 	log.Fatal(err)
				// }

				// const dpi = 72
				// mplusNormalFont, err := opentype.NewFace(tt, &opentype.FaceOptions{
				// 	Size:    24,
				// 	DPI:     dpi,
				// 	Hinting: font.HintingVertical,
				// })
				// if err != nil {
				// 	log.Fatal(err)
				// }
				// x, y := ebiten.CursorPosition()
				// text.Draw(screen, "haha", mplusNormalFont, x, y, color.White)
				// msg := "Hello, Ebiten!"
				// face := basicfont.Face7x13
				// text.Draw(screen, msg, face, 50, 50, color.White)
			}
		}
		// }
	}

	// Generate new balloons
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

	// Generate new bullets
	// if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {

	// 	for _, b := range g.bullets {
	// 		if !b.active {
	// 			b.active = true
	// 			x, _ := ebiten.CursorPosition()
	// 			b.x = float64(math.Mod(float64(x), screenWidth))
	// 			b.y = float64(screenHeight)
	// 			break
	// 		}
	// 	}
	// }
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
			// time.Sleep(300)
			// ebitenutil.DebugPrintAt(screen, "balloon", int(balloon.x), int(balloon.y))
		}
		balloon.shooted = 0
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("Score: %d", g.score))
	// for _, v :=  range idMap {
	if len(idMap) > 0 {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Pods scheduled on node %s", idMap[len(idMap)-1]), 0, 50)
	}
	
	// }
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

	// if err := ebiten.Run(Update, screenWidth, screenHeight, 5, "Balloon Shooter"); err != nil {
	// 	panic(err)
	// }

	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}



// package main

// import (
// 	"bufio"
// 	"fmt"
// 	"os/exec"
// 	"strings"
// )

// type Rule struct {
// 	Chain    string
// 	Target   string
// 	Protocol string
// 	Opt      string
// 	In       string
// 	Out      string
// 	Source   string
// 	Dest     string
// 	Extra    string
// 	next     *Rule
// }

// func main() {
// 	// Run the iptables command and get its output
// 	cmd := "kubectl exec -it kube-proxy-4pp9s -n kube-system -- sh -c 'iptables -S'"
// 	output, err := executeCommand(cmd)
// 	if err != nil {
// 		fmt.Println("Error executing command: ", err)
// 		return
// 	}

// 	// Parse the output and create a linked list of rules
// 	rules := createRulesList(output)

// 	// Print the list of rules
// 	for rule := rules; rule != nil; rule = rule.next {
// 		fmt.Printf("%s %s %s %s %s %s %s %s %s\n",
// 			rule.Chain, rule.Target, rule.Protocol, rule.Opt, rule.In, rule.Out, rule.Source, rule.Dest, rule.Extra)
// 	}
// }

// // executeCommand runs a command and returns its output
// func executeCommand(cmd string) (string, error) {
// 	output, err := exec.Command("kubectl", "exec", "-it", "kube-proxy-4pp9s", "-n", "kube-system", "--", "sh", "-c", "iptables -L").CombinedOutput()
// 	if err != nil {
// 		return "", err
// 	}

// 	return string(output), nil
// }

// // createRulesList parses the output of the iptables command and returns a linked list of rules
// func createRulesList(output string) *Rule {
// 	var head *Rule
// 	var tail *Rule

// 	scanner := bufio.NewScanner(strings.NewReader(output))
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		fields := strings.Fields(line)
// 		// if len(fields) < 6 || fields[0] != "Chain" {
// 		// 	continue
// 		// }

// 		if fields[0] == "Chain" {
// 			chainName := fields[1]
// 			scanner.Text()
// 			line := scanner.Text()

// 			if line == ""

// 			rule := &Rule{
// 				Chain:    chainName,
// 				Target:   fields[2],
// 				Protocol: fields[3],
// 				Opt:      fields[4],
// 				In:       fields[5],
// 				Out:      fields[6],
// 				Source:   fields[7],
// 				Dest:     fields[8],
// 				Extra:    strings.Join(fields[9:], " "),
// 				next:     nil,
// 			}
// 		}

// 		rule := &Rule{
// 			Chain:    fields[1],
// 			Target:   fields[2],
// 			Protocol: fields[3],
// 			Opt:      fields[4],
// 			In:       fields[5],
// 			Out:      fields[6],
// 			Source:   fields[7],
// 			Dest:     fields[8],
// 			Extra:    strings.Join(fields[9:], " "),
// 			next:     nil,
// 		}

// 		if head == nil {
// 			head = rule
// 			tail = rule
// 		} else {
// 			tail.next = rule
// 			tail = rule
// 		}
// 		fmt.Println(rule)
// 	}

// 	return head
// }
