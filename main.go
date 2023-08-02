package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"jf_requests/jf_requests"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"golang.org/x/term"
)

type Arguments struct {
	BaseUrl  string
	Username string
	Password string
	SeriesId string
	SeasonId string
	Name     string
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

	flag.Parse()

	return &args
}

// Checks, if all necessarry cli arguments are passed.
func CheckArguments(args *Arguments) (bool, string) {
	if args.BaseUrl == "" {
		return false, "No URL was given. See -h for more information"
	}

	if args.SeriesId == "" && args.Name == "" {
		return false, "No SeriesID or Name was given. See -h for more information."
	}

	return true, ""
}

func GetUsername(args *Arguments) string {
	if args.Username != "" {
		return args.Username
	}

	fmt.Printf("Username: ")
	reader := bufio.NewReader(os.Stdin)
	username, _ := reader.ReadString('\n')

	return strings.TrimSuffix(username, "\n")
}

func GetPassword(args *Arguments) string {
	if args.Password != "" {
		return args.Password
	}

	fmt.Printf("Password: ")
	bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()

	return string(bytePassword)
}

func PrintSummarry(episodes []jf_requests.Episode) bool {
	fmt.Println("The following Episodes will be downloaded:")
	color.Green("Series: %s", episodes[0].SeriesName)
	color.Green("Episodes:")
	for idx, episode := range episodes {
		color.Cyan("  %d. %s", idx, episode.Name)
	}

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
	response = strings.Split(response, "\n")[0]
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

func GetEpisodesToDownload(auth *jf_requests.AuthResponse, args *Arguments) ([]jf_requests.Episode, error) {

	seriesId := args.SeriesId
	if args.Name != "" {
		all, err := jf_requests.GetItemsForText(auth, args.BaseUrl, args.Name)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not get Items: %s", err))
		}

		if len(all) == 0 {
			return nil, errors.New("Nothing found for given searchtext")
		} else if len(all) == 1 {
			seriesId = all[0].Id
		} else {
			series, err := PrintItemSelection(all)
			if err != nil {
				return nil, err
			}

			seriesId = series.Id
		}

	}

	episodes, err := jf_requests.GetEpisodesFromId(auth.Token, args.BaseUrl, seriesId)
	if err != nil {
		return nil, err
	}

	if args.SeasonId != "" {
		return jf_requests.FilterEpisodesForSeason(episodes, args.SeasonId), nil
	}

	return episodes, nil

}

func main() {
	args := ParseCLIArgs()

	if status, msg := CheckArguments(args); !status {
		color.Red("Wrong Arguments: %s\n", msg)
		os.Exit(1)
	}

	username := GetUsername(args)
	password := GetPassword(args)

	creds, err := jf_requests.Authorize(args.BaseUrl, username, password)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	episodesToDownload, err := GetEpisodesToDownload(creds, args)
	if err != nil {
		color.Red("Failed to obtain episodes to download: %s", err)
		os.Exit(1)
	}

	shouldDownload := PrintSummarry(episodesToDownload)

	if shouldDownload {
		jf_requests.Download(episodesToDownload)
	}

}
