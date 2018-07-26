import * as React from "react";

import { observer } from "mobx-react";

import { Map, Marker, TileLayer, Popup } from "react-leaflet";

import { MissingCoordinate } from "./store";

interface Properties {
    coordinate?: MissingCoordinate;
}

@observer
class MapContainer extends React.Component<Properties, any> {
    ref: any;

    constructor(props: Properties) {
        super(props);
        this.ref = React.createRef();
        this.state = {
            width: 0,
            height: 0,
        };
    }

    updateDimensions = () => {
        const parent = this.ref.current.parentNode;
        if (parent) {
            this.setState({
                width: parent.offsetWidth,
                height: parent.offsetHeight,
            });
        }
    }

    componentDidMount() {
        this.updateDimensions();
        window.addEventListener("resize", this.updateDimensions);
    }

    componentWillUnmount() {
        window.removeEventListener("resize", this.updateDimensions);
    }

    render() {
        let position = null;
        if (this.props.coordinate) {
            let coord = this.props.coordinate.coordinate;
            position = [coord.lat, coord.lon];
        }

        return (
            <div ref={this.ref} style={{ height: this.state.height, width: this.state.width }}>
                <Map
                    style={{ height: this.state.height, width: this.state.width }}
                    center={position}
                    zoom={13}
                >
                    {position && <Marker position={position}>
                        <Popup>Test</Popup>
                    </Marker>}
                    <TileLayer
                      url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                      attribution="&copy; <a href=&quot;http://osm.org/copyright&quot;>OpenStreetMap</a> contributors"
                    />
                </Map>
            </div>
        );
    }
}

export default MapContainer;
