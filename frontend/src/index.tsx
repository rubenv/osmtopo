import * as React from 'react';
import * as ReactDOM from 'react-dom';

import { configure } from "mobx";

import registerServiceWorker from './registerServiceWorker';

import App from './App';
import Store from "./store";

import "./index.scss";
import "leaflet/dist/leaflet.css";

import L from "leaflet";

// fix default marker bug
// see https://github.com/PaulLeCam/react-leaflet/issues/255
delete L.Icon.Default.prototype._getIconUrl;
L.Icon.Default.mergeOptions({
    iconRetinaUrl: require('leaflet/dist/images/marker-icon-2x.png'),
    iconUrl: require('leaflet/dist/images/marker-icon.png'),
    shadowUrl: require('leaflet/dist/images/marker-shadow.png'),
});

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
