// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("Ephemeral integration test", func() {
	integration := os.Getenv("INTEGRATION")
	if strings.ToLower(integration) == "true" {
		var (
			code    string
			players []string
		)
		BeforeEach(func() {
			code = `listen(10000)
client_socket_id = regint()
acceptclientconnection(client_socket_id, 10000)
v = sint.read_from_socket(client_socket_id, 2)
v0 = v[0]
v1 = v[1]
sum = v0 + v1
revealed = sum.reveal()
print_ln("Result is %s", revealed)
resp = Array(1, cint)
resp[0] = revealed
cint.write_to_socket(client_socket_id, resp)`
			players = []string{
				"0-0", "1-0",
			}
		})
		It("receives secret shares directly in the http request", func() {
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA=",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA="}
			activation := getActivation(code)
			activation.SecretParams = secretParams
			verify := func(result Result) {
				decoded, _ := base64.StdEncoding.DecodeString(result.Response[0])
				Expect(string(decoded)).To(Equal("222"))
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("receives secret shares UUIDs", func() {
			amphoraUUIDs := []string{
				"1",
				"2"}
			activation := getActivation(code)
			activation.AmphoraParams = amphoraUUIDs
			verify := func(result Result) {
				decoded, _ := base64.StdEncoding.DecodeString(result.Response[0])
				Expect(string(decoded)).To(Equal("222"))
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("returns a 400 when wrongly encoded params are provided", func() {
			activation := getActivation(code)
			notValidBase64Params := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA"}
			activation.SecretParams = notValidBase64Params
			RunFailingMPCVerify(activation, players, "error decoding secret parameters: illegal base64 data at input byte 40", http.StatusBadRequest)
		})
		It("returns a 400 when both secret params and amphora params are specified", func() {
			activation := getActivation(code)
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA"}
			amphoraUUIDs := []string{
				"1",
				"2"}
			activation.SecretParams = secretParams
			activation.AmphoraParams = amphoraUUIDs
			RunFailingMPCVerify(activation, players, "either secret params or amphora secret share UUIDs must be specified, not both of them", http.StatusBadRequest)
		})
		It("returns a 400 when no parameters are specified", func() {
			activation := getActivation(code)
			RunFailingMPCVerify(activation, players, "either secret params or amphora secret share UUIDs must be specified, none of them given", http.StatusBadRequest)
		})
		It("returns a 503 when no compiled code is given", func() {
			// We neither supply code nor compile it and we also use generic container with no code inside. So the execution must fail.
			activation := getActivation("")
			amphoraUUIDs := []string{
				"1",
				"2"}
			players = []string{
				"0-2", "1-2",
			}
			activation.AmphoraParams = amphoraUUIDs
			RunFailingMPCVerify(activation, players, "error during MPC execution: error while executing the user code: exit status 134", http.StatusInternalServerError)
		})
		It("responds with multiple clear text parameters", func() {
			code = `listen(10000)
client_socket_id = regint()
acceptclientconnection(client_socket_id, 10000)
v = sint.read_from_socket(client_socket_id, 2)
v0 = v[0]
v1 = v[1]
sum = v0 + v1
revealed = sum.reveal()
print_ln("Result is %s", revealed)
resp = Array(2, cint)
resp[0] = revealed
resp[1] = revealed
cint.write_to_socket(client_socket_id, resp)`
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA=",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA="}
			activation := getActivation(code)
			activation.SecretParams = secretParams
			verify := func(result Result) {
				for j := 0; j < 2; j++ {
					decoded, _ := base64.StdEncoding.DecodeString(result.Response[j])
					Expect(string(decoded)).To(Equal("222"))
				}
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("responds with multiple secret-shared parameters", func() {
			code = `listen(10000)
client_socket_id = regint()
acceptclientconnection(client_socket_id, 10000)
v = sint.read_from_socket(client_socket_id, 2)
v0 = v[0]
v1 = v[1]
sum = v0 + v1
revealed = sum.reveal()
print_ln("Result is %s", revealed)
resp = Array(2, sint)
resp[0] = v0
resp[1] = v1
sint.write_to_socket(client_socket_id, resp)`
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA=",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA="}
			activation := getActivation(code)
			activation.SecretParams = secretParams
			activation.Output = OutputConfig{
				Type: SecretShare,
			}
			verify := func(result Result) {
				for j := 0; j < 2; j++ {
					Expect(result.Response[j]).To(Equal(secretParams[j]))
				}
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("uses bulk params from the request and responds back with normal param", func() {
			inputParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAABC/2ch+bIuVneIWcVfmkwEojwnAAAAAAAAAAAAAAAAAA=="}
			activation := getActivation(code)
			activation.SecretParams = inputParams
			activation.Output = OutputConfig{Type: PlainText}
			verify := func(result Result) {
				decoded, _ := base64.StdEncoding.DecodeString(result.Response[0])
				Expect(string(decoded)).To(Equal("222"))
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("uses bulk params from amphora and responds back with normal param", func() {
			amphoraUUID := []string{
				"3",
			}
			activation := getActivation(code)
			activation.AmphoraParams = amphoraUUID
			activation.Output = OutputConfig{
				Type: PlainText,
			}
			verify := func(result Result) {
				decoded, _ := base64.StdEncoding.DecodeString(result.Response[0])
				Expect(string(decoded)).To(Equal("222"))
			}
			RunMPCAndVerify(activation, players, verify)
		})
		It("writes the response to Amphora", func() {
			code = `listen(10000)
client_socket_id = regint()
acceptclientconnection(client_socket_id, 10000)
v = sint.read_from_socket(client_socket_id, 2)
v0 = v[0]
v1 = v[1]
sum = v0 + v1
revealed = sum.reveal()
print_ln("Result is %s", revealed)
resp = Array(2, sint)
resp[0] = v0
resp[1] = v1
sint.write_to_socket(client_socket_id, resp)`
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA=",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA="}
			activation := getActivation(code)
			activation.SecretParams = secretParams
			activation.Output = OutputConfig{
				Type: AmphoraSecret,
			}
			response := []string{activation.GameID}
			verify := func(result Result) {
				for j := 0; j < 2; j++ {
					Expect(result.Response[0]).To(Equal(response[0]))
				}
			}
			RunMPCAndVerify(activation, players, verify)
		})
		/*
		* There is an issue where the Discovery service cannot pair parallel / consecutive
		* ephemeral containers / executions.
		* The following test has to be disabled as long as this bug is not resolved.
		*
		* See https://rb-tracker.bosch.com/tracker01/browse/SPECS-2171 for more detail.
		 */
		XIt("starts 2 games in parallel", func() {
			// 2 games with 2 participants in each.
			playerGroup1 := []string{
				"0-0", "1-0",
			}
			playerGroup2 := []string{
				"0-1", "1-1",
			}
			secretParams := []string{
				"AAAAAAAAAAAAAAAAAAAAAHV5WQAAAAAAAAAAAAAAAAA=",
				"Qv9nIfmyLlZ3iFnFX5pMBKI8JwAAAAAAAAAAAAAAAAA="}

			wg := sync.WaitGroup{}
			verify := func(result Result) {
				defer func() {
					wg.Done()
				}()
				decoded, _ := base64.StdEncoding.DecodeString(result.Response[0])
				Expect(string(decoded)).To(Equal("222"))
			}
			activation1 := getActivation(code)
			activation1.SecretParams = secretParams
			activation2 := getActivation(code)
			activation2.SecretParams = secretParams
			wg.Add(2)
			go RunMPCAndVerify(activation1, playerGroup1, verify)
			wg.Add(2)
			go RunMPCAndVerify(activation2, playerGroup2, verify)
			wg.Wait()
		})
	}
})

func RunMPCAndVerify(activation *Activation, ids []string, verify func(Result)) {
	defer GinkgoRecover()
	rootDomain := getRootDomain()
	players := 2

	respCh := make(chan *http.Response, players)
	fmt.Printf("Executing the game with id %s\n", activation.GameID)
	wg := sync.WaitGroup{}
	for p := 0; p < players; p++ {
		act := *activation
		wg.Add(1)
		go sendRequest(ids[p], rootDomain, respCh, &wg, &act, true)
	}
	wg.Wait()
	for i := 0; i < players; i++ {
		resp := <-respCh
		result := Result{}
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		err := json.NewDecoder(resp.Body).Decode(&result)
		Expect(err).NotTo(HaveOccurred())
		verify(result)
	}
}

func RunFailingMPCVerify(activation *Activation, players []string, errorMsg string, statusCode int) {
	pl := len(players)
	rootDomain := getRootDomain()
	respCh := make(chan *http.Response, pl)
	fmt.Printf("Executing the game with id %s\n", activation.GameID)
	wg := sync.WaitGroup{}
	for p := 0; p < pl; p++ {
		act := *activation
		wg.Add(1)
		go sendRequest(players[p], rootDomain, respCh, &wg, &act, false)
	}
	wg.Wait()
	for i := 0; i < pl; i++ {
		resp := <-respCh
		Expect(resp.StatusCode).To(Equal(statusCode))
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(bodyBytes))
		Expect(string(bodyBytes)).To(Equal(errorMsg))
	}
}

func sendRequest(id string, domain string, respCh chan *http.Response, wg *sync.WaitGroup, activation *Activation, compile bool) {
	defer GinkgoRecover()
	svc := fmt.Sprintf("hellovc%s.default", id)
	url := func() string {
		if compile {
			return fmt.Sprintf("http://%s.%s/?compile=true", svc, domain)
		}
		return fmt.Sprintf("http://%s.%s", svc, domain)
	}()
	act, err := json.Marshal(activation)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(act))
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	fmt.Printf("Sending http %s request: %s\n", req.Method, url)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	select {
	case respCh <- resp:
		wg.Done()
	case <-time.After(1 * time.Minute):
		wg.Done()
		Expect("timeout happened").NotTo(Equal("timeout happened"))
	}

}

func getActivation(code string) *Activation {
	return &Activation{
		Code:   code,
		Output: OutputConfig{Type: PlainText},
		GameID: uuid.New().String(),
	}
}

func getRootDomain() string {
	// The url is the same for both players until we use the same cluster.
	var kubeConfig = os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		panic(errors.New("$KUBECONFIG env variable is not set"))
	}
	servingNs := "knative-serving"
	conf, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		panic(err)
	}
	kubeClient := kubernetes.NewForConfigOrDie(conf)

	configMap, err := kubeClient.CoreV1().ConfigMaps(servingNs).Get("config-domain", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	// Only a single domain name is allowed for the moment.
	if len(configMap.Data) != 1 {
		panic(errors.New("ambigous domain name defined"))
	}
	rootDomain := ""
	for k := range configMap.Data {
		rootDomain = k
	}
	if rootDomain == "" {
		panic(errors.New("invalid root domain"))
	}
	return rootDomain
}
