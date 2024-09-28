package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"jf_requests/jf_requests"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
)

const VERSION string = "v1.2.2"

type Arguments struct {
	BaseUrl  string
	Username string
	Password string
	SeriesId string
	SeasonId string
	Name     string
	Version  bool
}

// Parses the command line arguments and returns a struct containing all found arguments.
func ParseCLIArgs() *Arguments {
	var args = Arguments{}

	flag.StringVar(&args.BaseUrl, "url", "", "Base URL which points to the Jellyfin Instance")
	flag.StringVar(&args.SeriesId, "seriesid", "", "ID which points to the series which should be downloaded")
	flag.StringVar(&args.SeasonId, "seasonid", "", "If given, only the episodes with the provided season Id will be downloaded")
	flag.StringVar(&args.Username, "username", "", "Username used to login to the Jellyfin instance. If not provided, password will be prompted.")
	flag.StringVar(&args.Password, "password", "", "Passwort for the Jellyfin instance. If not provided, username will be prompted.")
	flag.StringVar(&args.Name, "name", "", "Name of the Show or Movie you want to download.")
	flag.BoolVar(&args.Version, "version", false, "Shows the Version Informations and Exit")

	flag.Parse()

	return &args
}

// Checks, if all necessarry cli arguments are passed.
func CheckArguments(args *Arguments) (bool, string) {
	if args.BaseUrl == "" {
		return false, "No URL was given. See -h for more information"
	}

	// Check if the URL was specified in the correct format.
	urlpattern := `https?\:\/\/[\d\w._-]+(:\d+)?\/?([/\d\w._-]*?)?$`
	match, err := regexp.Match(urlpattern, []byte(args.BaseUrl))
	if !match || err != nil {
		return false, "URL was supplied in the wrong pattern. The URL must be supplied like so: http(s)://myserver(:123)(/). Instead of the whole hostname, you can also specify the IPv4 address which is pointing to your Jellyfin server."
	}

	if args.SeriesId == "" && args.Name == "" {
		return false, "No SeriesID or Name was given. See -h for more information."
	}

	return true, ""
}

func GetUsername(args *Arguments) string {
	if args.Username != "" {
		return args.Username
	} else if username := os.Getenv("JF_USERNAME"); username != "" {
		return username
	}

	fmt.Printf("Username: ")
	reader := bufio.NewReader(os.Stdin)
	username, _ := reader.ReadString('\n')

	if runtime.GOOS == "windows" {
		return strings.TrimSuffix(username, "\r\n")

	}

	return strings.TrimSuffix(username, "\n")
}

func GetPassword(args *Arguments) string {
	if args.Password != "" {
		return args.Password
	} else if password := os.Getenv("JF_PASSWORD"); password != "" {
		return password
	}

	fmt.Printf("Password: ")
	bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()

	return string(bytePassword)
}

func GetConfirmation() bool {
	fmt.Print("Continue? y/n: ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y"
}

func PrintItemSelection(itemsToSelect []jf_requests.Item) (*jf_requests.Item, error) {
	fmt.Println("Found multiple Shows for the given Searchterm. Please Select the show you want to download:")

	for idx, show := range itemsToSelect {
		color.Cyan("  %d. %s", idx+1, show.Name)
	}

	fmt.Print("==> ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')

	if runtime.GOOS == "windows" {
		response = strings.TrimSuffix(response, "\r\n")
	} else {
		response = strings.TrimSuffix(response, "\n")
	}

	if selection, err := strconv.Atoi(response); err == nil {
		if selection < 0 || selection > len(itemsToSelect) {
			return nil, errors.New("Invalid Selection")
		}

		return &itemsToSelect[selection-1], nil
	} else {
		fmt.Println(err)
		return nil, errors.New("Only provide a single number")
	}
}

func DownloadSeries(auth *jf_requests.AuthResponse, baseurl string, item *jf_requests.Item, seasonId string) bool {
	series, err := jf_requests.GetSeriesFromItem(auth.Token, baseurl, item)
	if err != nil {
		color.Red("Failed to obtain Episode Information for given id: %s", err)
		return false
	}

	var selected_seasons []jf_requests.Season
	if seasonId != "" {
		if selected_season, geterr := series.GetSeasonForId(seasonId); geterr == nil {
			selected_seasons = []jf_requests.Season{*selected_season}
		} else {
			err = geterr
		}

	} else {
		selected_seasons, err = series.PrintAndGetSelection()
	}

	if err != nil {
		color.Red(err.Error())
		return false
	}

	confirm := series.PrintAndGetConfirmation(selected_seasons)

	if confirm {
		jf_requests.DownloadEpisodes(selected_seasons)
	}

	return true
}

func DownloadMovie(auth *jf_requests.AuthResponse, baseurl string, item *jf_requests.Item) bool {
	movie, err := jf_requests.GetMovieFromItem(auth, baseurl, item)
	if err != nil {
		color.Red("Failed to obtain Movie for given id: %s", err)
		return false
	}

	if movie.PrintAndGetConfirmation() {
		jf_requests.DownloadMovie(movie)
	} else {
		return false
	}

	return true
}

func Download(args *Arguments, auth *jf_requests.AuthResponse) bool {
	if args.SeriesId != "" {
		item, err := jf_requests.GetItemForId(auth, args.BaseUrl, args.SeriesId)
		if err != nil {
			color.Red("Failed to obtain items for given id: %s", err)
			return false
		}

		if item.Type == "Series" {
			return DownloadSeries(auth, args.BaseUrl, item, args.SeasonId)
		} else {
			return DownloadMovie(auth, args.BaseUrl, item)
		}

	} else if args.Name != "" {
		items, err := jf_requests.GetItemsForText(auth, args.BaseUrl, args.Name)
		if err != nil {
			color.Red("Failed to obtain Episode Information for given id: %s", err)
			return false
		}

		if len(items) == 0 {
			color.Yellow("Did not found anything for the given Searchterm on the Server.")
			return false
		}

		item, err := PrintItemSelection(items)
		if err != nil {
			color.Red(err.Error())
			return false
		}

		if item.Type == "Series" {
			return DownloadSeries(auth, args.BaseUrl, item, "")
		} else {
			return DownloadMovie(auth, args.BaseUrl, item)
		}

	}

	return false
}

func ShowVersionInfo() {
	fmt.Printf("JellyfinDownloader Version: %s\n", VERSION)
}

func main() {
	args := ParseCLIArgs()

	if args.Version {
		ShowVersionInfo()
		os.Exit(0)
	}

	if status, msg := CheckArguments(args); !status {
		color.Red("Wrong Arguments: %s\n", msg)
		os.Exit(1)
	}

	username := GetUsername(args)
	password := GetPassword(args)

	creds, err := jf_requests.Authorize(args.BaseUrl, username, password)
	if err != nil {
		color.Red("Authentication Failed!\n")
		color.Red("%s\n", err)
		os.Exit(1)
	}

	result := Download(args, creds)
	if !result {
		os.Exit(1)
	}
}
