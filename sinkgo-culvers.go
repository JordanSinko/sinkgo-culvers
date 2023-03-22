package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	cookiejar "github.com/saucesteals/fhttp/cookiejar"
	http2 "github.com/saucesteals/fhttp/http2"

	imap2 "github.com/emersion/go-imap"
	"github.com/jordansinko/sinkgo-mario/pkg/akamai"
	"github.com/jordansinko/sinkgo-mario/pkg/captcha"
)

var (
	version                = "N/A"
	defaultCaptchaProvider = "2cap|capmon"
	defaultCaptchaKey      = "<CAPTCHA_KEY>"
	defaultConcurrency     = 1
	defaultCatchall        = "gmail.com"
)

func main() {

	now := time.Now()

	randSource := rand.NewSource(now.UnixNano())
	randm := rand.New(randSource)

	lj := &lumberjack.Logger{Filename: `./logs/main.log`, MaxSize: 25, Compress: true}
	multiWriter := zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}, lj)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	log := zerolog.New(multiWriter).With().Timestamp().Logger()

	if now.Unix() > 1679957628 {
		err := errors.New("an unknown error occurred. contact dev")
		log.Panic().Err(err).Send()
	}

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	viper.SetDefault("captchaKey", defaultCaptchaKey)
	viper.SetDefault("captchaProvider", defaultCaptchaProvider)
	viper.SetDefault("catchall", defaultCatchall)
	viper.SetDefault("concurrency", defaultConcurrency)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			viper.SafeWriteConfig()
		} else {
			log.Panic().Err(err).Send()
		}
	}

	viper.WatchConfig()

	webhook := viper.GetString("webhook")
	catchall := viper.GetString("catchall")

	if catchall == defaultCatchall {
		panic(fmt.Errorf("please set catchall in config"))
	}

	captchaKey := viper.GetString("captchaKey")

	if captchaKey == defaultCaptchaKey {
		panic(fmt.Errorf("please set captcha key in config"))
	}

	captchaProvider := viper.GetString("captchaProvider")

	if captchaProvider == defaultCaptchaProvider {
		panic(fmt.Errorf("please set captcha provider in config"))
	}

	var captchaSolver captcha.CaptchaSolver

	if captchaProvider == "2cap" {
		captchaSolver = &captcha.TwoCaptcha{Key: captchaKey}
	} else if captchaProvider == "aycd" {
		captchaSolver = &captcha.Aycd{Key: captchaKey}
	} else {
		captchaSolver = &captcha.CapMon{Key: captchaKey}
	}

	captchaSolver.Initialize()

	var akamaiSolver akamai.AkamaiSolver

	akamaiSolver = &akamai.Flash{Authentication: "vLltFS9vFK8hJ13obkhjo9hKSzZr4VBA5unZSuH6"}

	// imapUsername := viper.GetString("imap_username")
	// ic, err := imap.New(viper.GetString("imap_host"), viper.GetInt("imap_port"), imapUsername, viper.GetString("imap_password"))

	// if err != nil {
	// 	log.Err(err).Send()
	// 	os.Exit(1)
	// }

	criteria := imap2.NewSearchCriteria()
	criteria.Header.Add("FROM", "admin@uber.com")
	criteria.Text = []string{"Welcome to Uber"}

	tm := NewTaskManager()
	pm := NewProxyManager()

	err := pm.Read()

	if err != nil {
		log.Err(err).Send()
		os.Exit(1)
	}

	type Stats struct {
		Attempts int
		Created  int
		Entered  int
		Winner   int
	}

	setConsoleTitle := func(title string) (int, error) {
		handle, err := syscall.LoadLibrary("Kernel32.dll")
		if err != nil {
			return 0, err
		}
		defer syscall.FreeLibrary(handle)
		proc, err := syscall.GetProcAddress(handle, "SetConsoleTitleW")
		if err != nil {
			return 0, err
		}
		r, _, err := syscall.Syscall(proc, 1, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))), 0, 0)
		return int(r), err
	}

	stats := make(chan Stats, 10)
	statsFlushed := make(chan bool)
	go func() {
		totalAttempts := 0
		totalCreated := 0
		totalEntered := 0
		totalWinners := 0

		els := []string{
			fmt.Sprintf(`Version: %s`, version),
			fmt.Sprintf(`Attempts: %d`, totalAttempts),
			fmt.Sprintf(`Created: %d`, totalCreated),
			fmt.Sprintf(`Entered: %d`, totalEntered),
			fmt.Sprintf(`Winners: %d`, totalWinners),
		}

		setConsoleTitle(strings.Join(els, `     `))

		for stat := range stats {

			totalAttempts = totalAttempts + stat.Attempts
			totalCreated = totalCreated + stat.Created
			totalEntered = totalEntered + stat.Entered
			totalWinners = totalWinners + stat.Winner

			els := []string{
				fmt.Sprintf(`Version: %s`, version),
				fmt.Sprintf(`Attempts: %d`, totalAttempts),
				fmt.Sprintf(`Created: %d`, totalCreated),
				fmt.Sprintf(`Entered: %d`, totalEntered),
				fmt.Sprintf(`Winners: %d`, totalWinners),
			}

			setConsoleTitle(strings.Join(els, `     `))
		}

		statsFlushed <- true
		close(stats)
	}()

	outputFlushed := make(chan bool)
	output := make(chan string, 10)
	go func() {
		fileName := fmt.Sprintf(`accounts-%d.txt`, time.Now().Unix())

		var file *os.File

		createdFile := false

		defer file.Close()

		for line := range output {

			if !createdFile {
				file, _ = os.Create(fileName)
				createdFile = true
			}

			newLine := fmt.Sprintf("%s\r\n", line)
			file.WriteString(newLine)
		}

		outputFlushed <- true
	}()

	baseUrl := `https://culvers.promo.eprize.com/`
	baseUrlObj, _ := url.Parse(baseUrl)

	abckPattern := regexp.MustCompile(`^[^~]+~(\-?\d)~[^~]+~(\-?\d)~.+`)
	abckUrlPattern := regexp.MustCompile(`type="text\/javascript"\s*src="([0-9a-zA-Z_/-]*)"`)
	cidPattern := regexp.MustCompile(`value="(cy.[^"]+)"`)
	resultPattern := regexp.MustCompile(`href="(\?cid=[^"]+)"`)

	taskHandler := func(ctx context.Context, wg *sync.WaitGroup, args ...interface{}) {
		defer wg.Done()

		id := fmt.Sprintf("%s", ctx.Value(TaskId{}))
		iterations := 0

		baseLog := log.With().Str("tid", id).Logger()

		baseLog.Print("starting task")

		shouldStop := false

		for {

			if shouldStop {
				break
			}

			iterations = iterations + 1

			select {
			case <-ctx.Done():
				shouldStop = true

			default:

				if iterations != 1 {
					dur := randm.Intn(5555) + 1
					baseLog.Printf("sleeping %dms", dur)
					time.Sleep(time.Duration(dur) * time.Millisecond)
				}

				p, err := pm.Lease(id)

				if err != nil {
					baseLog.Err(err).Send()
					continue
				}

				stats <- Stats{Attempts: 1}

				type Fake struct {
					Slug       string                `fake:"{adjective} {adjective} {noun}"`
					Random     string                `fake:"{number:1000,9999}"`
					First      string                `fake:"{firstname}"`
					Last       string                `fake:"{lastname}"`
					BirthDay   string                `fake:"{day}"`
					BirthMonth string                `fake:"{month}"`
					Phone      string                `fake:"{phone}"`
					Address    *gofakeit.AddressInfo `fake:"{address}"`
					State      string                `fake:"{randomstring:[AL,AR,AZ,CO,FL,GA,ID,IL,IN,IA,KS,KY,MI,MN,MO,NC,NE,ND,OH,SC,SD,TN,TX,UT,WI,WY]}"`
				}

				var f Fake

				gofakeit.Struct(&f)

				email := fmt.Sprintf("%s%s@%s", strings.ReplaceAll(strings.ToLower(f.Slug), " ", ""), f.Random, catchall)

				taskLog := baseLog.With().Str("email", email).Logger()

				proxy := &url.URL{
					Scheme: "http",
					Host:   fmt.Sprintf("%s:%s", p.host, p.port),
				}

				if p.username != "" {
					proxy.User = url.UserPassword(p.username, p.password)
				}

				cj, _ := cookiejar.New(nil)

				t1 := &http.Transport{Proxy: http.ProxyURL(proxy), GetTlsClientHelloSpec: func() *utls.ClientHelloSpec {
					spec, _ := utls.UTLSIdToSpec(utls.HelloChrome_106_Shuffle)
					return &spec
				}}

				t2, _ := http2.ConfigureTransports(t1)

				t2.Settings = []http2.Setting{
					{ID: http2.SettingHeaderTableSize, Val: 65536},
					{ID: http2.SettingEnablePush, Val: 0},
					{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
					{ID: http2.SettingInitialWindowSize, Val: 6291456},
					{ID: http2.SettingMaxHeaderListSize, Val: 262144},
				}

				t2.MaxHeaderListSize = 262144
				t2.InitialWindowSize = 6291456
				t2.HeaderTableSize = 65536

				client := &http.Client{Transport: t1, Jar: cj, CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				}}

				captchas := make(chan string, 2)
				requestCaptcha := func() {
					res, err := captchaSolver.SolveRecaptcha(&captcha.SolveRecaptchaOptions{
						Version:   "V2",
						Key:       "6LdmAf0SAAAAABgHCfB3ey-HxXCupdgZiuhwN21F",
						Url:       "https://culvers.promo.eprize.com/",
						Invisible: false,
					})

					taskLog.Printf("%v %v", res, err)

					if err != nil {
						taskLog.Err(err).Send()
						captchas <- ""
					} else {
						captchas <- res
					}

				}

				go requestCaptcha()

				var captcha1 string
				captchaTimeoutDur := 3 * time.Minute

				timeoutCh := time.After(captchaTimeoutDur)

				var cookieLookup = make(map[string]string)

				refreshCookieLookup := func() {
					for _, c := range client.Jar.Cookies(baseUrlObj) {
						cookieLookup[c.Name] = c.Value
					}
				}

				type RefreshAbckOptions struct {
					AbckUrl     string
					PageUrl     string
					MouseEvents int
					KeyEvents   int
				}

				type AbckDescription struct {
					Valid     bool
					Strong    bool
					Challenge bool
				}

				describeAbck := func(abck string) (*AbckDescription, error) {

					abckMatches := abckPattern.FindStringSubmatch(abck)

					if len(abckMatches) != 3 {
						return &AbckDescription{}, errors.New("abck was invalid or missing")
					}

					isStrong := abckMatches[1] == `0`
					isValid := isStrong || abckMatches[2] == `0`
					isChallenge := strings.Contains(abck, "||")

					return &AbckDescription{Strong: isStrong, Valid: isValid, Challenge: isChallenge}, nil
				}

				refreshAbck := func(options *RefreshAbckOptions) (*AbckDescription, error) {

					for _, c := range client.Jar.Cookies(baseUrlObj) {
						cookieLookup[c.Name] = c.Value
					}

					sensorData, err := akamaiSolver.GenerateSensorData(&akamai.GenerateSensorDataOptions{
						UserAgent:     `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`,
						ScriptUrl:     options.AbckUrl,
						ScriptEncoded: ``,

						Url:         options.PageUrl,
						Abck:        cookieLookup[`_abck`],
						Bmsz:        cookieLookup[`bm_sz`],
						MouseEvents: options.MouseEvents,
						KeyEvents:   options.KeyEvents,
					})

					//stats <- Stats{Sensors: 1}

					if err != nil {
						return &AbckDescription{}, err
					}

					reqBody := fmt.Sprintf(`{"sensor_data":"%s"}`, sensorData)
					req, err := http.NewRequest(http.MethodPost, options.AbckUrl, strings.NewReader(reqBody))

					if err != nil {
						return &AbckDescription{}, err
					}

					req.Header = http.Header{
						`sec-ch-ua`:          {`"Chromium";v="110", "Not A(Brand";v="24", "Google Chrome";v="110"`},
						`sec-ch-ua-platform`: {`"Windows"`},
						`sec-ch-ua-mobile`:   {`?0`},
						`user-agent`:         {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`},
						`content-type`:       {`text/plain;charset=UTF-8`},
						`accept`:             {`*/*`},
						`origin`:             {`https://culvers.promo.eprize.com`},
						`sec-fetch-site`:     {`same-origin`},
						`sec-fetch-mode`:     {`cors`},
						`sec-fetch-dest`:     {`empty`},
						`referer`:            {`https://culvers.promo.eprize.com/fishfrytry/`},
						`accept-encoding`:    {`gzip, deflate, br`},
						`accept-language`:    {`en-US,en;q=0.9`},
						http.HeaderOrderKey: {
							`content-length`,
							`sec-ch-ua`,
							`sec-ch-ua-platform`,
							`sec-ch-ua-mobile`,
							`user-agent`,
							`content-type`,
							`accept`,
							`origin`,
							`sec-fetch-site`,
							`sec-fetch-mode`,
							`sec-fetch-dest`,
							`referer`,
							`accept-encoding`,
							`accept-language`,
							`cookie`,
						},
						http.PHeaderOrderKey: {
							`:method`,
							`:authority`,
							`:scheme`,
							`:path`,
						},
					}

					res, err := client.Do(req)

					if err != nil {
						return &AbckDescription{}, err
					}

					defer res.Body.Close()

					refreshCookieLookup()

					abckInfo, _ := describeAbck(cookieLookup[`_abck`])

					taskLog.Debug().Bool("strong", abckInfo.Strong).Bool("valid", abckInfo.Valid).Bool("challenge", abckInfo.Challenge).Msg("refreshed cookie")

					return abckInfo, nil

				}

				var abckUrl string
				var cid string
				{
					refreshCookieLookup()

					reqLogger := taskLog.With().Str("scope", "request").Logger()

					req, err := http.NewRequest(`GET`, `https://culvers.promo.eprize.com/fishfrytry/`, nil)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					req.Header = http.Header{
						`sec-ch-ua`:                 {`"Chromium";v="110", "Not A(Brand";v="24", "Google Chrome";v="110"`},
						`sec-ch-ua-mobile`:          {`?0`},
						`sec-ch-ua-platform`:        {`"Windows"`},
						`upgrade-insecure-requests`: {`1`},
						`user-agent`:                {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`},
						`accept`:                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`},
						`sec-fetch-site`:            {`none`},
						`sec-fetch-mode`:            {`navigate`},
						`sec-fetch-user`:            {`?1`},
						`sec-fetch-dest`:            {`document`},
						`accept-encoding`:           {`gzip, deflate, br`},
						`accept-language`:           {`en-US,en;q=0.9`},
						http.HeaderOrderKey: {
							`sec-ch-ua`,
							`sec-ch-ua-mobile`,
							`sec-ch-ua-platform`,
							`upgrade-insecure-requests`,
							`user-agent`,
							`accept`,
							`sec-fetch-site`,
							`sec-fetch-mode`,
							`sec-fetch-user`,
							`sec-fetch-dest`,
							`accept-encoding`,
							`accept-language`,
						},
						http.PHeaderOrderKey: {
							`:method`,
							`:authority`,
							`:scheme`,
							`:path`,
						},
					}

					reqLogger.Print("getting cookies")
					res, err := client.Do(req)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						reqLogger.Err(fmt.Errorf("the response was not ok")).Send()
						continue
					}

					resBody, err := io.ReadAll(res.Body)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					//fmt.Println(string(resBody))

					abckUrlMatches := abckUrlPattern.FindStringSubmatch(string(resBody))

					if len(abckUrlMatches) == 0 {
						reqLogger.Err(errors.New("unable to determine akamai harvester url"))
						continue
					}

					abckUrlPath := abckUrlMatches[1]
					abckUrl = fmt.Sprintf(`%s%s`, baseUrl, abckUrlPath)

					akamaiSolver.WithScriptUrl(abckUrl)

					cidMatches := cidPattern.FindAllStringSubmatch(string(resBody), -1)

					if len(cidMatches) == 0 {
						reqLogger.Err(errors.New("unable to determine cid"))
						continue
					}

					cid = cidMatches[1][1]

				}

				{
					refreshCookieLookup()

					reqLogger := taskLog.With().Str("scope", "request").Logger()

					req, err := http.NewRequest(`GET`, abckUrl, nil)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					req.Header = http.Header{
						`sec-ch-ua`:          {`"Chromium";v="110", "Not A(Brand";v="24", "Google Chrome";v="110"`},
						`sec-ch-ua-mobile`:   {`?0`},
						`user-agent`:         {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`},
						`sec-ch-ua-platform`: {`"Windows"`},
						`accept`:             {`*/*`},
						`sec-fetch-site`:     {`same-origin`},
						`sec-fetch-mode`:     {`no-cors`},
						`sec-fetch-dest`:     {`script`},
						`referer`:            {`https://culvers.promo.eprize.com/fishfrytry/`},
						`accept-encoding`:    {`gzip, deflate, br`},
						`accept-language`:    {`en-US,en;q=0.9`},
						http.HeaderOrderKey: {
							`sec-ch-ua`,
							`sec-ch-ua-mobile`,
							`user-agent`,
							`sec-ch-ua-platform`,
							`accept`,
							`sec-fetch-site`,
							`sec-fetch-mode`,
							`sec-fetch-dest`,
							`referer`,
							`accept-encoding`,
							`accept-language`,
							`cookie`,
						},
						http.PHeaderOrderKey: {
							`:method`,
							`:authority`,
							`:scheme`,
							`:path`,
						},
					}

					reqLogger.Print("getting akamai cookies")
					res, err := client.Do(req)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						reqLogger.Err(fmt.Errorf("the response was not ok")).Send()
						continue
					}

					resBody, err := io.ReadAll(res.Body)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					// fmt.Println(string(resBody))

					abckScript := string(resBody)

					akamaiSolver.WithScriptContents(abckScript)

				}

				foundStrong := false

				for i := 1; i <= 66; i++ {

					dur := randm.Intn(3333) + 1
					time.Sleep(time.Duration(dur) * time.Millisecond)

					desc, _ := refreshAbck(&RefreshAbckOptions{
						AbckUrl:     abckUrl,
						PageUrl:     `https://culvers.promo.eprize.com/fishfrytry/`,
						MouseEvents: 0,
						KeyEvents:   0,
					})

					if desc.Strong {
						foundStrong = true
						break
					}

				}

				if !foundStrong {
					err := errors.New("not able to generate a good abck")
					taskLog.Err(err).Send()
					continue
				}

				taskLog.Print("waiting for captcha to complete")

				select {
				case res := <-captchas:
					captcha1 = res
				case <-timeoutCh:
					err := fmt.Errorf("captcha failed to return within %s", captchaTimeoutDur)
					taskLog.Err(err).Send()
					continue
				}

				if captcha1 == "" {
					continue
				}

				var resultUrl string
				{
					refreshCookieLookup()

					reqLogger := taskLog.With().Str("scope", "request").Logger()

					reqForm := &url.Values{}
					reqForm.Add(`first_name`, f.First)
					reqForm.Add(`last_name`, f.Last)
					reqForm.Add(`email`, email)
					reqForm.Add(`address1`, f.Address.Street)
					reqForm.Add(`address2`, ``)
					reqForm.Add(`city`, f.Address.City)
					reqForm.Add(`state`, fmt.Sprintf(`US-%s`, f.State))
					reqForm.Add(`zip`, f.Address.Zip)
					reqForm.Add(`age.birth_month`, f.BirthMonth)
					reqForm.Add(`age.birth_day`, f.BirthDay)
					reqForm.Add(`age.birth_year`, fmt.Sprintf("%d", gofakeit.IntRange(1990, 1975)))
					reqForm.Add(`primary_opt_in__Magic`, `1`)
					reqForm.Add(`gigya_network`, ``)
					reqForm.Add(`social_uid`, ``)
					reqForm.Add(`signed_request`, ``)
					reqForm.Add(`access_token`, ``)
					reqForm.Add(`g-recaptcha-response`, captcha1)
					reqForm.Add(`cid`, cid)

					reqBody := reqForm.Encode()

					//fmt.Println(reqBody)
					req, err := http.NewRequest(`POST`, `https://culvers.promo.eprize.com/fishfrytry/`, strings.NewReader(reqBody))

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					req.Header = http.Header{
						`cache-control`:             {`max-age=0`},
						`sec-ch-ua`:                 {`"Chromium";v="110", "Not A(Brand";v="24", "Google Chrome";v="110"`},
						`sec-ch-ua-mobile`:          {`?0`},
						`sec-ch-ua-platform`:        {`"Windows"`},
						`upgrade-insecure-requests`: {`1`},
						`origin`:                    {`https://culvers.promo.eprize.com`},
						`content-type`:              {`application/x-www-form-urlencoded`},
						`user-agent`:                {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`},
						`accept`:                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`},
						`sec-fetch-site`:            {`same-origin`},
						`sec-fetch-mode`:            {`navigate`},
						`sec-fetch-user`:            {`?1`},
						`sec-fetch-dest`:            {`document`},
						`referer`:                   {`https://culvers.promo.eprize.com/fishfrytry/`},
						`accept-encoding`:           {`gzip, deflate, br`},
						`accept-language`:           {`en-US,en;q=0.9`},
						http.HeaderOrderKey: {
							`content-length`,
							`cache-control`,
							`sec-ch-ua`,
							`sec-ch-ua-mobile`,
							`sec-ch-ua-platform`,
							`upgrade-insecure-requests`,
							`origin`,
							`content-type`,
							`user-agent`,
							`accept`,
							`sec-fetch-site`,
							`sec-fetch-mode`,
							`sec-fetch-user`,
							`sec-fetch-dest`,
							`referer`,
							`accept-encoding`,
							`accept-language`,
							`cookie`,
						},
						http.PHeaderOrderKey: {
							`:method`,
							`:authority`,
							`:scheme`,
							`:path`,
						},
					}

					reqLogger.Print("submitting entry")
					res, err := client.Do(req)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						reqLogger.Err(fmt.Errorf("the response was not ok")).Send()
						continue
					}

					resBody, err := io.ReadAll(res.Body)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					//fmt.Println(string(resBody))

					resultsMatches := resultPattern.FindStringSubmatch(string(resBody))

					if len(resultsMatches) == 0 {
						reqLogger.Err(errors.New("unable to determine results url"))
						continue
					}

					resultsPath := resultsMatches[1]

					resultUrl = fmt.Sprintf(`https://culvers.promo.eprize.com/fishfrytry/%s`, resultsPath)

				}

				{
					refreshCookieLookup()

					reqLogger := taskLog.With().Str("scope", "request").Logger()

					req, err := http.NewRequest(`GET`, resultUrl, nil)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					req.Header = http.Header{
						`sec-ch-ua`:                 {`"Chromium";v="110", "Not A(Brand";v="24", "Google Chrome";v="110"`},
						`sec-ch-ua-mobile`:          {`?0`},
						`sec-ch-ua-platform`:        {`"Windows"`},
						`upgrade-insecure-requests`: {`1`},
						`user-agent`:                {`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36`},
						`accept`:                    {`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`},
						`sec-fetch-site`:            {`same-origin`},
						`sec-fetch-mode`:            {`navigate`},
						`sec-fetch-user`:            {`?1`},
						`sec-fetch-dest`:            {`document`},
						`referer`:                   {`https://culvers.promo.eprize.com/fishfrytry/`},
						`accept-encoding`:           {`gzip, deflate, br`},
						`accept-language`:           {`en-US,en;q=0.9`},
						http.HeaderOrderKey: {
							`sec-ch-ua`,
							`sec-ch-ua-mobile`,
							`sec-ch-ua-platform`,
							`upgrade-insecure-requests`,
							`user-agent`,
							`accept`,
							`sec-fetch-site`,
							`sec-fetch-mode`,
							`sec-fetch-user`,
							`sec-fetch-dest`,
							`referer`,
							`accept-encoding`,
							`accept-language`,
							`cookie`,
						},
						http.PHeaderOrderKey: {
							`:method`,
							`:authority`,
							`:scheme`,
							`:path`,
						},
					}

					reqLogger.Print("checking result")
					res, err := client.Do(req)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						reqLogger.Err(fmt.Errorf("the response was not ok")).Send()
						continue
					}

					resBody, err := io.ReadAll(res.Body)

					if err != nil {
						reqLogger.Err(err).Send()
						continue
					}

					text := string(resBody)
					isWinner := !strings.Contains(text, "SO CLOSE, BUT NO CATCH!")

					stats <- Stats{Entered: 1}

					if !isWinner {
						taskLog.Warn().Msg("you are not a winner :(")
					} else {
						taskLog.Info().Msg("you are a winner!")
						stats <- Stats{Winner: 1}

						filename := fmt.Sprintf("./logs/%s.txt", email)
						os.WriteFile(filename, resBody, 0644)

						if webhook != "" {
							reqJson := map[string]interface{}{
								"content": nil,
								"embeds": []map[string]interface{}{
									{
										"title": "Winner: Culvers Giveaway",
										"color": 65300,
										"fields": []map[string]interface{}{
											{
												"name":  "email",
												"value": email,
											},
										},
									},
								},
							}

							reqBody, _ := json.Marshal(reqJson)
							req, _ := http.NewRequest(http.MethodPost, webhook, bytes.NewBuffer(reqBody))

							req.Header.Set("content-type", "application/json")

							http.DefaultClient.Do(req)

						}

					}

				}

			}
		}

		baseLog.Print("done with work")

	}

	i := 0
	for i < viper.GetInt("concurrency") {
		tm.AddTask(taskHandler)
		i = i + 1
	}

	tm.StartTasks()
	tm.WaitGroup.Wait()

	close(output)

	<-outputFlushed
	log.Print("all tasks done")

}
