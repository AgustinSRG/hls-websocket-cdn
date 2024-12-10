const path = require('path');

module.exports = {
    mode: "production",
    entry: path.resolve(__dirname, "src", "index.ts"),
    output: {
        filename: "hls-websocket-cdn.js",
        path: path.resolve(__dirname, 'dist.webpack'),
        library: "HlsWebSocketCdn",
    },
    resolve: {
        extensions: [".webpack.js", ".web.js", ".ts", ".js"],
    },
    module: {
        rules: [{ test: /\.ts$/, loader: "ts-loader" }]
    },
    plugins: [],
}
