<script>
    export let label, placeholder;
</script>

<main>
    <p>{label}</p>
    <input type="text" {placeholder}>
</main>

<style>
    input {
        font-size: 12pt;
        padding: 10px;
        margin: 5px 0;
        color: white;
        background-color: black;
        border: 2px solid white;
        width: 200px;
    }

    p {
        font-size: 14pt;
        margin: 10px 0;
    }
</style>
