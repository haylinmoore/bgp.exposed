<script>
    export let label;
</script>

<main>
    <button type="submit">{label}</button>
</main>

<style>
    button {
        font-size: 12pt;
        padding: 10px 15px;
        border: 2px solid white;
        background-color: black;
        color: white;
    }

    button:hover {
        color: black;
        background-color: white;
    }
</style>
