import { observable, runInAction, autorun, action } from "mobx";

export interface MissingCoordinate {
    coordinate: Coordinate;
    suggestions: { [key: string]: Array<Suggestion> };
}

interface Coordinate {
    lat: number;
    lon: number;
}

interface Config {
    layers: Array<Layer>;
}

export interface Suggestion {
    id: number;
    name: string;
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
    @observable public initialized: boolean = false;
    @observable public missing: number = 0;

    @observable public coordinate?: MissingCoordinate;
    @observable public config: Config;

    @observable.shallow
    public topologies: { [key: string]: Topology } = {};

    @observable public highlightLayer: string;
    @observable public highlightFeature: number;

    public startPoll() {
        this.pollStatus();
        setInterval(() => this.pollStatus(), 1000);
        autorun(() => {
            if (this.initialized && this.missing > 0 && !this.coordinate) {
                this.loadCoordinate();
            }
        });
    }

    private async pollStatus() {
        const response = await fetch("/api/status", {
            credentials: "same-origin",
            method: "GET",
        });
        if (!response.ok) {
            throw new Error("Failed: " + response.status);
        }
        const result = await response.json();
        runInAction(() => {
            this.updating = result.running;
            this.initialized = result.initialized;
            this.missing = result.missing || 0;
            this.config = result.config;
        });
    }

    private async loadCoordinate() {
        console.log("Loading coordinate");
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
        this.topologies[layer+"/"+id] = result;
    }

    @action
    public hoverFeature(layer: string, feature: number) {
        this.highlightLayer = layer;
        this.highlightFeature = feature;
    }
}

export default Store;
