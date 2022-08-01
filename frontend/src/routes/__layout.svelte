<script>
    import Logo from "../components/Logo.svelte";
</script>

<main>
    <Logo/>
    <p><slot name="banner"/></p>
    <slot/>
</main>

<style>
    main {
        margin: 50px auto;
    }
</style>
