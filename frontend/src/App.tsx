import * as React from "react";

import { observer, Provider } from "mobx-react";

import {
    Col, Container, Row,
    Navbar, NavbarBrand, Nav, NavItem,
    FormGroup, Label, Input,
} from "reactstrap";

import Store, { Layer, Suggestion } from "./store";

import MapContainer from "./MapContainer";

interface AppProperties {
    store: Store;
}

@observer
class App extends React.Component<AppProperties, any> {
    private renderLoading() {
        const { store } = this.props;

        return <Container className="h-100">
            <Row className="h-100 align-items-center">
                <Col className="text-center">
                    <h1>Initializing...</h1>
                    { store.updating && <p>Geometry data is being updated.</p> }
                </Col>
            </Row>
            </Container>;
    }

    private renderSuggestion(layer: Layer, suggestion: Suggestion) {
        return <FormGroup check={true} className="suggestion">
            <Label check={true}>
                <Input type="radio" />{' ' + suggestion.name }
            </Label>
            </FormGroup>;
    }

    private renderLayer(layer: Layer) {
        const { store } = this.props;
        const info = store.coordinate;
        if (!info) {
            return;
        }

        const suggestions = info.suggestions[layer.id];

        return (
            <div>
                <h2>{layer.name}</h2>
                { (!suggestions || !suggestions.length) && <em>No suggestions</em> }
                { (suggestions && suggestions.length) && suggestions.map((suggestion) => this.renderSuggestion(layer, suggestion)) }
            </div>
        )
    }

    private renderCoordinate() {
        const { store } = this.props;
        const info = store.coordinate;
        if (!info) {
            return;
        }

        const layers = store.config.layers;
        return (
            <div>
                <div className="coord">
                    {info.coordinate.lat}
                    /
                    {info.coordinate.lon}
                </div>
                { layers.map((l) => this.renderLayer(l)) }
            </div>
        );
    }

    public render() {
        const { store } = this.props;

        if (!store.initialized) {
            return this.renderLoading();
        }

        return (
            <Provider store={store}>
                <section className="app">
                    <Navbar color="dark" dark={true}>
                        <NavbarBrand href="/">OSMtopo</NavbarBrand>
                        <Nav navbar>
                            <NavItem>Missing: {store.missing}</NavItem>
                        </Nav>
                    </Navbar>
                    <section className="main">
                        <div className="map">
                            <MapContainer />
                        </div>
                        <div className="coordinate">
                            { store.coordinate && this.renderCoordinate() }
                        </div>
                    </section>
                </section>
            </Provider>
        );
    }
}

export default App;
