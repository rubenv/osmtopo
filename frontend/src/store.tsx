import { observable, runInAction, autorun } from "mobx";

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

class Store {
    @observable public updating: boolean = false;
    @observable public initialized: boolean = false;
    @observable public missing: number = 0;

    @observable public coordinate?: MissingCoordinate;
    @observable public config: Config;

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
        });
    }
}

export default Store;
