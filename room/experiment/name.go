package experiment

import (
	"math/rand"
	"time"
)

var (
	adjectives = []string{
		"autumn", "hidden", "bitter", "misty", "silent", "empty", "dry", "dark",
		"summer", "icy", "delicate", "quiet", "white", "cool", "spring", "winter",
		"patient", "twilight", "dawn", "crimson", "wispy", "weathered", "blue",
		"billowing", "broken", "cold", "damp", "falling", "frosty", "green",
		"long", "late", "lingering", "bold", "little", "morning", "muddy", "old",
		"red", "rough", "still", "small", "sparkling", "throbbing", "shy",
		"wandering", "withered", "wild", "black", "young", "holy", "solitary",
		"fragrant", "aged", "snowy", "proud", "floral", "restless", "divine",
		"polished", "purple", "lively", "nameless", "lucky", "oddball", "crystal",
	}

	nouns = []string{
		"waterfall", "river", "breeze", "moon", "rain", "wind", "sea", "morning",
		"snow", "lake", "sunset", "pine", "shadow", "leaf", "dawn", "glitter",
		"forest", "hill", "cloud", "meadow", "sun", "glade", "bird", "brook",
		"butterfly", "bush", "dew", "dust", "field", "fire", "flower", "firefly",
		"feather", "grass", "haze", "mountain", "night", "pond", "darkness",
		"snowflake", "silence", "sound", "sky", "shape", "surf", "thunder",
		"violet", "water", "wildflower", "wave", "water", "resonance", "sun",
		"wood", "dream", "cherry", "tree", "fog", "frost", "voice", "paper",
		"frog", "smoke", "star", "silver", "brass", "gold", "copper", "rhythm",
	}
)

// GenerateExperimentName creates a memorable experiment identifier
// in the format "adjective-noun"
func GenerateExperimentName() string {
	// Set random seed based on current time
	rand.New(rand.NewSource(time.Now().UnixNano()))

	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]

	return adj + "-" + noun
}

// GenerateExperimentID creates a unique experiment identifier by combining
// the memorable name with a timestamp
func GenerateExperimentID() string {
	timestamp := time.Now().UTC().Format("20060102-150405")
	return GenerateExperimentName() + "-" + timestamp
}
