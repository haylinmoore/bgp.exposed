<script>

    export let maxLength = 30
    export let list = [];
    
    let val = "";
    let i = 0;
    let overflowed = false
    while (val.length < maxLength && i < list.length){
        val += list[i]
        if (i+1 < list.length && val.length < maxLength){
            val += ", "
        } else if (val.length >= maxLength) {
            val += "..."
            overflowed = true
        }
        i++;
    }
    function click(){
        alert(list.join(", "))
    }
 </script>

<div class={overflowed?"overflowed":""} on:click={click}>{val}</div>

<style>
    .overflowed:hover {
        text-decoration: underline;
        cursor: pointer;
    }
</style>