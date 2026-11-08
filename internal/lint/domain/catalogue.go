package domain

type RuleCatalogue struct {
	Packs []CataloguePack `json:"packs"`
}

type CataloguePack struct {
	Name  PackName        `json:"name"`
	Rules []CatalogueRule `json:"rules"`
}

type CatalogueRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}
