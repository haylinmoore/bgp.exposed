<script>
    export let value = "";
    export let label = "";
    export let placeholder = "";
    export let rightPadding = false;
    export let bottomPadding = false;
    export let wide = false;
    export let number = false;
    export let required = false;
    export let disabled = false;
</script>

<main class:rightPadding class:bottomPadding>
    <p>{label}</p>
    <!-- Can't use a ternary here since type bindings don't work right -->
    {#if number}
        <input type="number" {placeholder} {required} {disabled} class:wide bind:value>
    {:else}
        <input type="text" {placeholder} {required} {disabled} class:wide bind:value>
    {/if}
</main>

<style>
    input {
        padding: 10px;
        margin: 5px 0;
        color: white;
        background-color: black;
        border: 2px solid white;
        width: 183px;
    }

    p {
        font-size: 12pt;
        margin: 5px 0;
    }

    .rightPadding {
        padding-right: 10px;
    }

    .bottomPadding {
        padding-bottom: 5px;
    }

    .wide {
        width: 400px;
    }
</style>
