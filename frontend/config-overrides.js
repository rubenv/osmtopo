const { compose } = require("react-app-rewired");

const rewireTypescript = require("react-app-rewire-typescript");
const rewireSass = require("react-app-rewire-scss");

module.exports = function override(config, env) {
    const rewires = compose(
        rewireSass,
        rewireTypescript,
    );

    return rewires(config, env);
}
