package script

import (
	"fmt"
	"github.com/threagile/threagile/pkg/input"
	"github.com/threagile/threagile/pkg/security/types"
	"gopkg.in/yaml.v3"
	"io/fs"
	"path/filepath"
	"strings"
)

type RiskRule struct {
	types.RiskRule
	category      types.RiskCategory
	supportedTags []string
	script        *Script
}

func (what *RiskRule) Init() *RiskRule {
	return what
}

func (what *RiskRule) ParseFromData(text []byte) (*RiskRule, error) {
	categoryError := yaml.Unmarshal(text, &what.category)
	if categoryError != nil {
		return nil, categoryError
	}

	var rule struct {
		Category      string         `yaml:"category"`
		SupportedTags []string       `yaml:"supported-tags"`
		Script        map[string]any `yaml:"risk"`
	}

	ruleError := yaml.Unmarshal(text, &rule)
	if ruleError != nil {
		return nil, ruleError
	}

	what.supportedTags = rule.SupportedTags
	script, scriptError := new(Script).ParseScript(rule.Script)
	if scriptError != nil {
		return nil, scriptError
	}

	what.script = script

	return what, nil
}

func (what *RiskRule) Category() *types.RiskCategory {
	return &what.category
}

func (what *RiskRule) SupportedTags() []string {
	return what.supportedTags
}

func (what *RiskRule) GenerateRisks(parsedModel *types.Model) ([]*types.Risk, error) {
	if what.script == nil {
		return nil, fmt.Errorf("no script found in risk rule")
	}

	newScope, scopeError := what.script.NewScope(&what.category)
	if scopeError != nil {
		return nil, scopeError
	}

	modelError := newScope.SetModel(parsedModel)
	if modelError != nil {
		return nil, modelError
	}

	newRisks, errorLiteral, riskError := what.script.GenerateRisks(newScope)
	if riskError != nil {
		msg := make([]string, 0)
		msg = append(msg, fmt.Sprintf("error generating risks: %v\n", riskError))

		if len(errorLiteral) > 0 {
			msg = append(msg, fmt.Sprintf("in:\n%v\n", new(input.Strings).IndentPrintf(1, errorLiteral)))
		}

		return nil, fmt.Errorf(strings.Join(msg, "\n"))
	}

	return newRisks, nil
}

func (what *RiskRule) GetTechnicalAssetsByRiskID(parsedModel *types.Model, riskID string) ([]*types.TechnicalAsset, error) {
	newScope, scopeError := what.script.NewScope(&what.category)
	if scopeError != nil {
		return nil, scopeError
	}

	modelError := newScope.SetModel(parsedModel)
	if modelError != nil {
		return nil, modelError
	}

	if what.script == nil {
		return nil, fmt.Errorf("no script found in risk rule")
	}

	genericAssets, lookupError := what.script.GetTechnicalAssetsByRiskID(newScope, riskID)
	if lookupError != nil {
		return nil, lookupError
	}

	assets := make([]*types.TechnicalAsset, 0)
	for _, genericAsset := range genericAssets {
		asset := new(types.TechnicalAsset)
		assetData, assetError := yaml.Marshal(genericAsset)
		if assetError != nil {
			return nil, assetError
		}

		unmarshalError := yaml.Unmarshal(assetData, asset)
		if unmarshalError != nil {
			return nil, unmarshalError
		}

		assets = append(assets, asset)
	}

	return assets, nil
}

func (what *RiskRule) Load(fileSystem fs.FS, path string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return nil
	}

	loadError := what.loadRiskRule(fileSystem, path)
	if loadError != nil {
		return loadError
	}

	return nil
}

func (what *RiskRule) loadRiskRule(fileSystem fs.FS, filename string) error {
	scriptFilename := filepath.Clean(filename)

	ruleData, ruleReadError := fs.ReadFile(fileSystem, scriptFilename)
	if ruleReadError != nil {
		return fmt.Errorf("error reading data category: %w\n", ruleReadError)
	}

	_, parseError := what.ParseFromData(ruleData)
	if parseError != nil {
		return fmt.Errorf("error parsing scripts from %q: %w\n", scriptFilename, parseError)
	}

	return nil
}
