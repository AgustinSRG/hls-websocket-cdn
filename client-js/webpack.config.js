const path = require('path');
const webpack = require('webpack');

module.exports = {
    mode: "production",
    entry: "./src/index.ts",
    output: {
        filename: "hls-websocket-cdn.js",
        path: path.resolve(__dirname, 'dist.webpack'),
        library: "HlsWebsocketCdn",
    },
    resolve: {
        extensions: [".webpack.js", ".web.js", ".ts", ".js"],
    },
    module: {
        rules: [{ test: /\.ts$/, loader: "ts-loader" }]
    },
    plugins: [
        new webpack.ProvidePlugin({
            Buffer: ['buffer', 'Buffer'],
        }),
    ],
}
