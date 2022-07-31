<script>
    import Input from "../components/Input.svelte";
    import Checkbox from "../components/Checkbox.svelte";
    import Button from "../components/Button.svelte";
</script>

<main>
    <div class="logo">
        <p class="bgp">bgp</p>
        <p class="exposed">.exposed</p>
    </div>

    <h3>New BGP Session</h3>
    <Input label="Our ASN" placeholder="65530"/>
    <Input label="Your ASN" placeholder="65530"/>
    <Checkbox label="ADD_PATH?"/>
    <Button label="Submit"/>

    <h3>Announcements</h3>
    <Checkbox label="Full table?"/>
    <Input label="Prefix" placeholder="192.0.2.0/24"/>
    <Button label="Update"/>
</main>

<style>
    main {
        max-width: 900%;
        margin: 50px auto 0;
    }

    .logo p {
        font-size: 28pt;
        padding: 0;
        margin: 0;
        width: max-content;
    }

    .exposed {
        color: black;
        background-color: white;
        margin-top: 5px !important;
        padding: 10px !important;
    }
</style>
