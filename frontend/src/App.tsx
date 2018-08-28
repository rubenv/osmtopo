import * as React from "react";

import { observer } from "mobx-react";

import {
    Col, Container, Row,
    Navbar, NavbarBrand, Nav, NavItem,
    FormGroup, Label, Input, Button, Alert
} from "reactstrap";

import Store, { ExportStatus, Layer, Suggestion } from "./store";

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

    private renderDone() {
        const { store } = this.props;

        return <Container className="h-100">
                { store.export && this.renderExport(store.export) }
                <Row className="h-100 align-items-center">
                    <Col className="text-center">
                        <h1>All done!</h1>
                    </Col>
                </Row>
            </Container>;
    }

    private renderSuggestion(layer: Layer, suggestion: Suggestion) {
        return <FormGroup
            check={true}
            className="suggestion"
            key={suggestion.id}
            onMouseEnter={this.hoverSuggestion(layer, suggestion)}
            onMouseLeave={this.unhoverSuggestion}
        >
            <Label check={true}>
                <Input type="radio" name={layer.id} onChange={this.selectSuggestion(layer, suggestion)} />
                <span className="admin_level">{suggestion.admin_level}</span>
                {' ' + suggestion.name }
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
        const matched = info.matched[layer.id];
        const name = info.matchnames[layer.id];

        return (
            <div key={layer.id}>
                <h2>{layer.name}</h2>
                { matched && <em>Matched: {name}</em> }
                { !matched && (!suggestions || !suggestions.length) && <em>No suggestions</em> }
                { suggestions && suggestions.map((suggestion) => this.renderSuggestion(layer, suggestion)) }
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
                <Button
                    color="primary"
                    disabled={store.selectionCount == 0}
                    onClick={this.saveSelections}
                >Save</Button>
                {" "}
                <Button
                    color="danger"
                    onClick={this.deleteMissing}
                >Delete</Button>
            </div>
        );
    }

    private renderSpinner() {
        return (
            <div className="center-block">
                <div className="spinner"></div>
            </div>
        );
    }

    private renderExport(e: ExportStatus) {
        if (!e.running && !e.error) {
            return <div />;
        }
        return (
            <div className="export-status">
                { e.running && <Alert color="primary">Export running...</Alert> }
                { e.error != "" && <Alert color="danger">Export failed: {e.error}</Alert> }
            </div>
        );
    }

    private hoverSuggestion = (layer: Layer, suggestion: Suggestion) => () => {
        this.props.store.hoverFeature(layer.id, suggestion.id);
    }

    private unhoverSuggestion = () => {
        this.props.store.hoverFeature("", 0);
    }

    private selectSuggestion = (layer: Layer, suggestion: Suggestion) => () => {
        this.props.store.selectSuggestion(layer.id, suggestion.id);
    }

    private saveSelections = () => {
        this.props.store.saveSelections();
    }

    private deleteMissing = () => {
        this.props.store.deleteMissing();
    }

    public render() {
        const { store } = this.props;

        if (!store.initialized) {
            return this.renderLoading();
        }

        if (!store.missing) {
            return this.renderDone();
        }

        return (
            <section className="app">
                <Navbar color="dark" dark={true}>
                    <NavbarBrand href="/">OSMtopo</NavbarBrand>
                    <Nav navbar>
                        <NavItem>Missing: {store.missing}</NavItem>
                    </Nav>
                </Navbar>
                <section className="main">
                    <div className="map">
                        <MapContainer store={store} />
                    </div>
                    <div className="coordinate">
                        { !store.loading && store.coordinate && this.renderCoordinate() }
                        { store.loading && this.renderSpinner() }
                    </div>
                    { store.export && this.renderExport(store.export) }
                </section>
            </section>
        );
    }
}

export default App;
