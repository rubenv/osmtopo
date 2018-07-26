import * as React from 'react';
import * as ReactDOM from 'react-dom';

import { configure } from "mobx";

import registerServiceWorker from './registerServiceWorker';

import App from './App';
import Store from "./store";

import "./index.scss";
import "leaflet/dist/leaflet.css";

configure({ enforceActions: true });

const store = new Store();

const renderApp = () => {
    ReactDOM.render(
        <App store={store} />,
        document.getElementById("root") as HTMLElement
    );
};

renderApp();
if (module.hot) {
    module.hot.accept("./App", renderApp);
}

store.startPoll();

registerServiceWorker();
