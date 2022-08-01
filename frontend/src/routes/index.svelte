<script>
    import Input from "../components/Input.svelte";
    import Checkbox from "../components/Checkbox.svelte";
    import Button from "../components/Button.svelte";
</script>

<main>
    <p class="banner" >BGP.exposed is a ...</p>

    <div class="row">
        <form action="/session">
            <h3>New BGP Session</h3>
            <Input required label="ASN" placeholder="65530" number/>
            <Input required label="IP" placeholder="192.0.2.19"/>
            <Input required label="Our ASN" placeholder="65510" number/>
            <Input required label="MD5 Password" placeholder="Optional"/>
            <Checkbox required bottomPadding label="ADD_PATH?"/>
            <Button label="Submit"/>
        </form>
    </div>
</main>
