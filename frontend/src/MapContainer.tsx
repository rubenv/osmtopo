import * as React from "react";

import { Map, TileLayer } from "react-leaflet";

interface Properties {

}

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
        const position = [51.505, -0.09];

        return (
            <div ref={this.ref} style={{ height: this.state.height, width: this.state.width }}>
                <Map
                    style={{ height: this.state.height, width: this.state.width }}
                    center={position}
                    zoom={13}
                >
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
