package main

import (
	"flag"
	"fmt"
	"log"

	"sparkdance/assets-jsonrpc/internal/rpcserver"
)

func main() {
	var root string
	var operation string
	var payloadJSON string
	var manualOverridesPath string
	var manualTokensPath string
	var hotCurrentPath string

	flag.StringVar(&root, "root", "../..", "Trust Wallet assets repository root")
	flag.StringVar(&operation, "operation", "", "tokenlist config operation")
	flag.StringVar(&payloadJSON, "payload-json", "", "operation payload JSON")
	flag.StringVar(&manualOverridesPath, "manual-overrides-file", rpcserver.DefaultTokenListManualOverridesPath, "manual override JSON path, relative to --root unless absolute")
	flag.StringVar(&manualTokensPath, "manual-tokens-file", rpcserver.DefaultTokenListManualTokensPath, "manual token JSON path, relative to --root unless absolute")
	flag.StringVar(&hotCurrentPath, "hot-current-file", rpcserver.DefaultTokenListHotCurrentPath, "current hot list JSON path, relative to --root unless absolute")
	flag.Parse()

	op, err := rpcserver.ParseTokenListConfigOperation(operation)
	if err != nil {
		log.Fatal(err)
	}

	result, err := rpcserver.ApplyTokenListConfigOperation(root, manualOverridesPath, manualTokensPath, hotCurrentPath, op, payloadJSON)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"operation=%s manual_overrides_updated=%t manual_tokens_updated=%t hot_current_updated=%t\n",
		op,
		result.ManualOverridesUpdated,
		result.ManualTokensUpdated,
		result.HotCurrentUpdated,
	)
}
