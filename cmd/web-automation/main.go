// Command web-automation keeps a website data folder in sync with a podcast
// RSS feed and with Bluesky posts. Instagram is not included: plug an
// implementation of social.InstagramSource in your own main (see the README).
package main

import "github.com/Mundo-Dolphins/mundodolphins-web-automation/cli"

func main() {
	cli.Execute(cli.New(cli.Options{}))
}
