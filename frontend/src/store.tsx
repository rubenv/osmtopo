import { observable, runInAction } from "mobx";

class Store {
    @observable public updating: boolean = false;
    @observable public initialized: boolean = false;
    @observable public missing: number = 0;

    public startPoll() {
        this.pollStatus();
        setInterval(() => this.pollStatus(), 1000);
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
        });
    }
}

export default Store;
