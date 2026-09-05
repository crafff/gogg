package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/crafff/gogg/packages/riotapi"
)

func main() {
	puuid := flag.String("puuid", "", "Riot PUUID")
	gameName := flag.String("game-name", "", "Riot game name")
	tagLine := flag.String("tag-line", "", "Riot tag line")
	count := flag.Int("count", 100, "ranked matches to request")
	output := flag.String("output", "data/user_interactions.csv", "output CSV")
	secrets := flag.String("secrets", "../../deploy/secrets/dev.enc.yaml", "SOPS encrypted config")
	flag.Parse()
	if *puuid == "" || *gameName == "" || *tagLine == "" {
		fatalf("puuid, game-name and tag-line are required")
	}
	apiKey, err := decryptAPIKey(*secrets)
	if err != nil {
		fatalf("load Riot API key: %v", err)
	}
	client := riotapi.NewClient(apiKey, "https://na1.api.riotgames.com", "https://americas.api.riotgames.com")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	account, err := client.GetAccountByRiotID(ctx, *gameName, *tagLine)
	if err != nil {
		fatalf("verify Riot account: %v", err)
	}
	if account.Puuid != *puuid {
		fatalf("Riot ID resolves to a different PUUID")
	}
	ids, err := client.GetMatchIDsByPUUID(ctx, *puuid, 420, 0, 0, 0, *count)
	if err != nil {
		fatalf("fetch match IDs: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fatalf("create output directory: %v", err)
	}
	file, err := os.Create(*output)
	if err != nil {
		fatalf("create output: %v", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	header := []string{"match_id", "game_start_ts", "region", "queue_id", "patch", "puuid", "champion_id", "champion_name", "position", "tier_at_match", "win", "kills", "deaths", "assists", "gold_earned", "total_damage_dealt_to_champions", "vision_score", "time_played"}
	if err := writer.Write(header); err != nil {
		fatalf("write header: %v", err)
	}
	positions := map[string]int{}
	written := 0
	for _, id := range ids {
		detail, err := client.GetMatchDetail(ctx, id)
		if err != nil {
			fatalf("fetch match detail %s: %v", id, err)
		}
		for _, participant := range detail.Info.Participants {
			if participant.Puuid != *puuid {
				continue
			}
			position := participant.TeamPosition
			if position == "" {
				position = participant.IndividualPosition
			}
			positions[position]++
			patchParts := strings.Split(detail.Info.GameVersion, ".")
			patch := detail.Info.GameVersion
			if len(patchParts) >= 2 {
				patch = patchParts[0] + "." + patchParts[1]
			}
			row := []string{
				detail.Metadata.MatchID,
				time.UnixMilli(detail.Info.GameStartTimestamp).UTC().Format(time.RFC3339Nano),
				"NA1", strconv.Itoa(detail.Info.QueueID), patch, participant.Puuid,
				strconv.Itoa(participant.ChampionID), participant.ChampionName, position, "",
				strconv.FormatBool(participant.Win), strconv.Itoa(participant.Kills),
				strconv.Itoa(participant.Deaths), strconv.Itoa(participant.Assists),
				strconv.Itoa(participant.GoldEarned), strconv.Itoa(participant.TotalDamageDealtToChampions),
				strconv.Itoa(participant.VisionScore), strconv.Itoa(participant.TimePlayed),
			}
			if err := writer.Write(row); err != nil {
				fatalf("write match: %v", err)
			}
			written++
			break
		}
	}
	if err := writer.Error(); err != nil {
		fatalf("flush output: %v", err)
	}
	fmt.Printf("fetched=%d written=%d positions=%v output=%s\n", len(ids), written, positions, *output)
}

func decryptAPIKey(path string) (string, error) {
	command := exec.Command("sops", "--decrypt", "--extract", `["riot"]["api_key"]`, "--output-type", "json", path)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	var key string
	if err := json.Unmarshal(output, &key); err != nil {
		key = strings.TrimSpace(string(output))
	}
	if !strings.HasPrefix(key, "RGAPI-") {
		return "", fmt.Errorf("decrypted value is not a Riot API key")
	}
	return key, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
