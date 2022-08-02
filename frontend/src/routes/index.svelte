<script>
    import Input from "../components/Input.svelte";
    import Button from "../components/Button.svelte";
    import {onMount} from "svelte";
    import {connect, createSession} from "../api.js";

    onMount(connect);
    let peerASN = 65530;
    let peerIP = "192.0.2.1";
    let localASN = 65510;
</script>

<main>
    <p class="banner">BGP.exposed is a ...</p>

    <div class="row">
        <form on:submit|preventDefault={() => createSession(peerASN, peerIP, localASN)}>
            <h3>New BGP Session</h3>
            <Input required label="ASN" placeholder="65530" number bind:value={peerASN}/>
            <Input required label="IP" placeholder="192.0.2.19" bind:value={peerIP}/>
            <Input required bottomPadding label="Our ASN" placeholder="65510" number bind:value={localASN}/>
            <Button label="Submit"/>
        </form>
    </div>
</main>
