import * as React from "react";

import { observer } from "mobx-react";

import { Map, Marker, TileLayer, GeoJSON } from "react-leaflet";

import Store from "./store";

import * as topojson from "topojson";

interface Properties {
    store: Store;
}

@observer
class MapContainer extends React.Component<Properties, any> {
    ref: any;

    topoKey: string;
    bounds?: {};
    geoData?: {};

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
        const { store } = this.props;

        let position = null;
        if (store.coordinate) {
            let coord = store.coordinate.coordinate;
            position = [coord.lat, coord.lon];
        }

        const topoKey = store.highlightLayer+"/"+store.highlightFeature;
        if (this.topoKey != topoKey) {
            this.geoData = undefined;
            this.bounds  = undefined;
            this.topoKey = topoKey;
        }
        if (!this.geoData && store.topologies[topoKey]) {
            const topo = store.topologies[topoKey];
            if (topo) {
                const obj = topo.objects[store.highlightFeature];
                if (obj) {
                    this.geoData = topojson.feature(topo, obj);

                    const bbox = topo.bbox;
                    this.bounds = [
                        [bbox[1], bbox[0]],
                        [bbox[3], bbox[2]],
                    ];
                }
            }
        }

        const style = {
            color: "#FF0000",
            weight: 3,
            opacity: 0.5,
        };

        return (
            <div ref={this.ref} style={{ height: this.state.height, width: this.state.width }}>
                <Map
                    style={{ height: this.state.height, width: this.state.width }}
                    bounds={this.bounds}
                    center={position}
                    zoom={13}
                >
                    {position && <Marker position={position} />}
                    {this.geoData && <GeoJSON key={this.topoKey} data={this.geoData} style={style} />}
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
