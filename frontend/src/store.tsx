import { observable, runInAction, autorun, action, computed } from "mobx";

interface Status {
    running: boolean;
    export: ExportStatus;
    initialized: boolean;
    missing: number;
    config: Config;
}

export interface ExportStatus {
    running: boolean;
    error: string;
}

export interface MissingCoordinate {
    coordinate: Coordinate;
    suggestions: { [key: string]: Array<Suggestion> };
    matched: { [key: string]: boolean };
    matchnames: { [key: string]: string };
    matchids: { [key: string]: number };
}

interface Coordinate {
    lat: number;
    lon: number;
}

interface Config {
    layers: Array<Layer>;
    rules: Array<MatchRule>;
}

interface MatchRule {
    match: { [key: string]: number };
    restrict: { [key: string]: Array<number> };
}

export interface Suggestion {
    id: number;
    name: string;
    admin_level: number;
    disabled: boolean;
}

export interface Layer {
    id:   string;
    name: string;
    admin_levels: Array<number>;
}

interface Topology {
    bbox: {};
    objects: {};
}

class Store {
    @observable public updating: boolean = false;
    @observable public export: ExportStatus;
    @observable public initialized: boolean = false;
    @observable public missing: number = 0;
    @observable public loading: boolean = false;

    @observable public coordinate?: MissingCoordinate;
    @observable public config: Config;

    @observable.shallow
    public topologies: { [key: string]: Topology } = {};

    @observable public highlightLayer: string;
    @observable public highlightFeature: number;

    @observable public selected: { [key: string]: number } = {};

    public startPoll() {
        this.pollStatus();
        setInterval(() => this.pollStatus(), 1000);
        autorun(() => {
            if (this.initialized && this.missing > 0 && !this.coordinate) {
                this.loadCoordinate();
            }
        });
        autorun(() => {
            if (!this.initialized) {
                return;
            }

            let coord = this.coordinate;
            if (!coord) {
                return;
            }

            let rules = this.config.rules;
            if (!rules) {
                return;
            }

            for (var layer in coord.suggestions) {
                var s = coord.suggestions[layer];
                s.forEach((suggestion: Suggestion) => {
                    runInAction(() => {
                        suggestion.disabled = false;
                        rules.forEach((rule: MatchRule) => {
                            let matches = true;
                            for (let l in rule.match) {
                                if (!coord) {
                                    continue;
                                }
                                let value = coord.matchids[l] || this.selected[l] || 0;
                                matches = matches && rule.match[l] == value;
                            }
                            if (!matches) {
                                return;
                            }

                            for (let r in rule.restrict) {
                                if (r == layer) {
                                    let allowed = rule.restrict[r];
                                    if (allowed.indexOf(suggestion.admin_level) < 0) {
                                        suggestion.disabled = true;
                                    }
                                }
                            }
                        });
                    });
                });
            }
        });
    }

    private async pollStatus() {
        const response = await fetch("/api/status", {
            credentials: "same-origin",
            method: "GET",
        });
        if (!response.ok) {
            return;
        }
        const result = await response.json();
        this.updateStatus(result);
    }

    @action
    private updateStatus(status: Status) {
        this.updating = status.running;
        this.export = status.export;
        this.initialized = status.initialized;
        this.missing = status.missing || 0;
        this.config = status.config;
    }

    @action
    private async loadCoordinate() {
        this.loading = true;
        const response = await fetch("/api/coordinate", {
            credentials: "same-origin",
            method: "GET",
        });
        if (!response.ok) {
            throw new Error("Failed: " + response.status);
        }
        const result = await response.json();
        runInAction(() => {
            this.coordinate = result;
            this.loading = false;
            if (!this.coordinate) {
                return;
            }

            this.topologies = {};
            for (var layer in this.coordinate.suggestions) {
                var s = this.coordinate.suggestions[layer];
                s.forEach((suggestion: Suggestion) => {
                    this.loadTopology(layer, suggestion.id);
                });
            }
        });
    }

    private async loadTopology(layer: string, id: number) {
        const response = await fetch("/api/topo/" + layer + "/" + id, {
            credentials: "same-origin",
            method: "GET",
        });
        if (!response.ok) {
            throw new Error("Failed: " + response.status);
        }
        const result = await response.json();
        runInAction(() => {
            this.topologies[layer+"/"+id] = result;
        });
    }

    @action
    public hoverFeature(layer: string, feature: number) {
        this.highlightLayer = layer;
        this.highlightFeature = feature;
    }

    @action
    public selectSuggestion(layer: string, feature: number) {
        this.selected[layer] = feature;
    }

    @computed
    get selectionCount(): number {
        return Object.keys(this.selected).length;
    }

    @action
    public async saveSelections() {
        this.loading = true;
        const response = await fetch("/api/add", {
            credentials: "same-origin",
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(this.selected),
        });
        if (!response.ok) {
            throw new Error("Failed: " + response.status);
        }
        runInAction(() => {
            this.selected = {};
            this.loadCoordinate();
        });
    }

    @action
    public async deleteMissing() {
        if (!this.coordinate) {
            return;
        }
        this.loading = true;
        const response = await fetch("/api/delete", {
            credentials: "same-origin",
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(this.coordinate.coordinate),
        });
        if (!response.ok) {
            throw new Error("Failed: " + response.status);
        }
        runInAction(() => {
            this.loadCoordinate();
        });
    }
}

export default Store;
