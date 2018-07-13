import * as React from "react";

import { observer, Provider } from "mobx-react";

import { Col, Container, Row } from "reactstrap";

import Store from "./store";

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

    public render() {
        const { store } = this.props;

        if (!store.initialized) {
            return this.renderLoading();
        }

        return (
            <Provider store={store}>
                <div className="h-100">
                    App here!
                </div>
            </Provider>
        );
    }
}

export default App;
