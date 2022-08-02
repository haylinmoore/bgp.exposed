let ws;

export function connect() {
    ws = new WebSocket("ws://localhost:8080/ws"); // window.location.host + "/ws"

    ws.onopen = function () {
        console.log("ws connected");
    }
}

function send(o) {
    console.log(o)
    ws.send(JSON.stringify(o));
}

export function createSession(peerASN, peerIP, localASN) {
    send({
        type: "CreateRequest",
        data: {
            peerASN: peerASN,
            peerIP: peerIP,
            localASN: localASN
        }
    });
}

export function updateSession(md5Password, addPath, fullTable) {
    send({
        type: "UpdateRequest",
        data: {
            md5Password: md5Password,
            addPath: addPath,
            fullTable: fullTable,
        }
    });
}

export function deleteAnnouncement(route) {
    // TODO
}

export function routeDataListener() {
    ws.addEventListener("message", (e) => {
        if (e.type === "RouteData") {
            for (const prefix of e.data.prefixes) {
                let route = {
                    id: prefix.id,
                    prefix: prefix.prefix,
                    path: e.data.asPath,
                    nexthop: e.data.nextHop,
                    communities: [], // TODO
                    origin: e.data.origin,
                    rpki: "valid",
                    irr: true,
                }
                // TODO: Push route to table
            }
        }
    })
}

export function addAnnouncement(prefix, path, nexthop, origin) {
    ws.send(JSON.stringify({
        type: "RouteData",
        data: {
            prefixes: [{prefix: prefix, id: new Date().getUTCMilliseconds()}],
            asPath: path,
            origin: origin,
            nextHop: nexthop,
        },
    }))
}
