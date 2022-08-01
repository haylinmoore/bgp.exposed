<script>
    import Input from "../components/Input.svelte";
    import Button from "../components/Button.svelte";
    import {onMount} from "svelte";
    import {ws} from "../stores.js";

    onMount(() => {
        ws = new WebSocket("wss://" + window.location.host);
    })

    let peerASN, peerIP, localASN;

    function createSession() {
        $ws.send(JSON.stringify({
            type: "CreateRequest",
            data: {
                peerASN: peerASN,
                peerIP: peerIP,
                localASN: localASN
            }
        }));
    }
</script>

<main>
    <p class="banner">BGP.exposed is a ...</p>

    <div class="row">
        <form on:submit|preventDefault={() => createSession()}>
            <h3>New BGP Session</h3>
            <Input required label="ASN" placeholder="65530" number bind:value={peerASN}/>
            <Input required label="IP" placeholder="192.0.2.19" bind:value={peerIP}/>
            <Input required bottomPadding label="Our ASN" placeholder="65510" number bind:value={localASN}/>
            <Button label="Submit"/>
        </form>
    </div>
</main>
